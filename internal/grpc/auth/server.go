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
	) (token string, err error)

	IsAdmin(
		ctx context.Context,
		userID int64,
	) (bool, error)
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

	token, err := s.auth.Login(ctx, req.GetEmail(), req.GetPassword(), int(req.GetAppId()))
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}

		return nil, status.Error(codes.Internal, "failed to login")
	}

	return &authv1.LoginResponse{Token: token}, nil
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
