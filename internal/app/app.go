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
	refreshTokenTTL time.Duration,
	hasher auth.Hasher,
	cipher auth.Cipher,
	storage auth.Storage,
) *App {
	authService := auth.New(log, tokenTTL, refreshTokenTTL, hasher, cipher, storage)
	grpcApp := grpcapp.New(log, grpcPort, authService)

	return &App{
		GRPCServer: grpcApp,
	}
}
