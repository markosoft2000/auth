package authgrpc

import (
	"context"

	"github.com/google/uuid"
	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"

	"google.golang.org/grpc"
)

type Auth interface {
	RegisterNewUser(
		ctx context.Context,
		email string,
		password string,
	) (userID uuid.UUID, err error)

	Login(
		ctx context.Context,
		email string,
		password string,
		appID uuid.UUID,
		ip string,
	) (
		accessToken string,
		refreshToken string,
		err error,
	)

	Logout(
		ctx context.Context,
		userID uuid.UUID,
		appID uuid.UUID,
		allApp bool,
	) error

	IsAdmin(
		ctx context.Context,
		userID uuid.UUID,
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
	) (id uuid.UUID, err error)

	RemoveApp(ctx context.Context, appID uuid.UUID) error
}

type serverAPI struct {
	authv1.UnimplementedAuthServer
	auth Auth
}

// RegisterServerAPI registers the auth service to the gRPC server
func Register(gRPC *grpc.Server, auth Auth) {
	authv1.RegisterAuthServer(gRPC, &serverAPI{auth: auth})
}
