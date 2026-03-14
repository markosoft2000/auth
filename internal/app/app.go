package app

import (
	"log/slog"

	grpcapp "github.com/markosoft2000/auth/internal/app/grpc"
	"github.com/markosoft2000/auth/internal/service/auth"
)

type App struct {
	GRPCServer *grpcapp.App
}

func New(
	log *slog.Logger,
	grpcPort int,
	authServiceCfg auth.AuthConfig,
) *App {

	authService := auth.New(log, authServiceCfg)
	grpcApp := grpcapp.New(log, grpcPort, authService)

	return &App{
		GRPCServer: grpcApp,
	}
}
