package auth

import (
	"context"

	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type serverAPI struct {
	authv1.UnimplementedAuthServer
}

// RegisterServerAPI registers the auth service to the gRPC server
func Register(gRPC *grpc.Server) {
	authv1.RegisterAuthServer(gRPC, &serverAPI{})
}

func (s *serverAPI) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	// MOCK LOGIC
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	return &authv1.LoginResponse{
		Token: "mock-token-for-" + req.GetEmail(),
	}, nil
}

func (s *serverAPI) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	return &authv1.RegisterResponse{
		UserId: 123, // Mock ID
	}, nil
}

func (s *serverAPI) IsAdmin(ctx context.Context, req *authv1.IsAdminRequest) (*authv1.IsAdminResponse, error) {
	return &authv1.IsAdminResponse{
		IsAdmin: true, // Everyone is admin in mock mode!
	}, nil
}
