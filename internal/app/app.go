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
	cfg auth.AuthCfg,
	hasher auth.Hasher,
	cipher auth.Cipher,
	storage auth.Storage,
	dbPinger grpcapp.Pinger,
) *App {
	authService := auth.New(log, cfg, hasher, cipher, storage)
	grpcApp := grpcapp.New(log, dbPinger, grpcPort, authService)

	return &App{
		GRPCServer: grpcApp,
	}
}
