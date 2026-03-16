package app

import (
	"log/slog"
	"time"

	grpcapp "github.com/markosoft2000/auth/internal/app/grpc"
	"github.com/markosoft2000/auth/internal/service/auth"
)

type App struct {
	GRPCServer *grpcapp.App
}

func New(
	log *slog.Logger,
	grpcPort int,
	tokenTTL time.Duration,
	hasher auth.Hasher,
	userSaver auth.UserSaver,
	userProvider auth.UserProvider,
	appProvider auth.AppProvider,
) *App {
	authService := auth.New(log, tokenTTL, hasher, userSaver, userProvider, appProvider)
	grpcApp := grpcapp.New(log, grpcPort, authService)

	return &App{
		GRPCServer: grpcApp,
	}
}
