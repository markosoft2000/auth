package authgrpc

import (
	"context"
	"errors"

	"github.com/markosoft2000/auth/internal/service/auth"
	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Auth interface {
	RegisterNewUser(
		ctx context.Context,
		email string,
		password string,
	) (userID int64, err error)

	Login(
		ctx context.Context,
		email string,
		password string,
		appID int,
		ip string,
	) (
		accessToken string,
		refreshToken string,
		err error,
	)

	IsAdmin(
		ctx context.Context,
		userID int64,
	) (bool, error)

	RefreshToken(
		ctx context.Context,
		refreshToken string,
		ip string,
	) (
		newAccessToken string,
		newRefreshToken string,
		err error,
	)

	AddApp(
		ctx context.Context,
		appName string,
		appSecret []byte,
	) (id int, err error)

	RemoveApp(ctx context.Context, appId int) error
}

type serverAPI struct {
	authv1.UnimplementedAuthServer
	auth Auth
}

// RegisterServerAPI registers the auth service to the gRPC server
func Register(gRPC *grpc.Server, auth Auth) {
	authv1.RegisterAuthServer(gRPC, &serverAPI{auth: auth})
}

func (s *serverAPI) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	userID, err := s.auth.RegisterNewUser(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}

		return nil, status.Error(codes.Internal, "failed to register user")
	}

	return &authv1.RegisterResponse{UserId: userID}, nil
}

func (s *serverAPI) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	if req.GetAppId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}

	accessToken, refreshToken, err := s.auth.Login(
		ctx,
		req.GetEmail(),
		req.GetPassword(),
		int(req.GetAppId()),
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

func (s *serverAPI) IsAdmin(ctx context.Context, req *authv1.IsAdminRequest) (*authv1.IsAdminResponse, error) {
	if req.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	isAdmin, err := s.auth.IsAdmin(ctx, req.GetUserId())
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

func (s *serverAPI) AddApp(ctx context.Context, req *authv1.AddAppRequest) (*authv1.AddAppResponse, error) {
	id, err := s.auth.AddApp(ctx, req.GetName(), req.GetSecret())
	if err != nil {
		if errors.Is(err, auth.ErrAppExists) {
			return nil, status.Error(codes.AlreadyExists, "app already exists")
		}

		return nil, status.Error(codes.Internal, "failed to add app")
	}

	return &authv1.AddAppResponse{Id: int32(id)}, nil
}

func (s *serverAPI) RemoveApp(ctx context.Context, req *authv1.RemoveAppRequest) (*authv1.RemoveAppResponse, error) {
	err := s.auth.RemoveApp(ctx, int(req.GetId()))
	if err != nil {
		if errors.Is(err, auth.ErrAppNotFound) {
			return nil, status.Error(codes.NotFound, "app not found")
		}

		return nil, status.Error(codes.Internal, "failed to remove app")
	}

	return &authv1.RemoveAppResponse{}, nil
}
