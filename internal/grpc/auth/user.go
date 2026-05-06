package authgrpc

import (
	"context"
	"errors"

	"github.com/markosoft2000/auth/internal/service/auth"
	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *serverAPI) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	userID, err := s.auth.RegisterNewUser(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}

		return nil, status.Error(codes.Internal, "failed to register user")
	}

	return &authv1.RegisterResponse{UserId: userID.String()}, nil
}

func (s *serverAPI) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	if req.GetAppId() == "" {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}

	appID, err := convertStringToUUIDv7(req.GetAppId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid app ID")
	}

	accessToken, refreshToken, err := s.auth.Login(
		ctx,
		req.GetEmail(),
		req.GetPassword(),
		appID,
		req.GetIp(),
	)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}

		return nil, status.Error(codes.Internal, "failed to login")
	}

	return &authv1.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *serverAPI) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	appID, err := convertStringToUUIDv7(req.GetAppId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid app ID")
	}

	userID, err := convertStringToUUIDv7(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	err = s.auth.Logout(
		ctx,
		userID,
		appID,
		req.AllApp,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to logout")
	}

	return &authv1.LogoutResponse{}, nil
}

func (s *serverAPI) IsAdmin(ctx context.Context, req *authv1.IsAdminRequest) (*authv1.IsAdminResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := convertStringToUUIDv7(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	isAdmin, err := s.auth.IsAdmin(ctx, userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}

		return nil, status.Error(codes.Internal, "failed to check the admin role")
	}

	return &authv1.IsAdminResponse{IsAdmin: isAdmin}, nil
}

func (s *serverAPI) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	accessToken, refreshToken, err := s.auth.RefreshToken(ctx, req.GetRefreshToken(), req.GetIp())
	if err != nil {
		if errors.Is(err, auth.ErrRefreshTokenNotFound) || errors.Is(err, auth.ErrInvalidIpAddress) {
			return nil, status.Error(codes.Unauthenticated, "token not found")
		}

		return nil, status.Error(codes.Internal, "failed to refresh access token")
	}

	return &authv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
