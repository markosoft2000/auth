package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcapp "github.com/markosoft2000/auth/internal/app"
	"github.com/markosoft2000/auth/internal/config"
	"github.com/markosoft2000/auth/internal/lib/hasher/argon2"
	"github.com/markosoft2000/auth/internal/routes"
	"github.com/markosoft2000/auth/internal/storage"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	r := routes.NewRouter(log, cfg.HTTPServer.Timeout)

	// HTTP Server Configuration
	srv := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.HTTPServer.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTPServer.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.HTTPServer.IdleTimeout) * time.Second,
	}

	go func() {
		log.Info("http server starting", slog.String("addr", cfg.HTTPServer.Address))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// GRPC Server Configuration
	hasher := argon2.New(
		cfg.Hasher.Memory,
		cfg.Hasher.Iterations,
		cfg.Hasher.Parallelism,
		cfg.Hasher.SaltLength,
		cfg.Hasher.KeyLength,
	)

	storage := storage.NewMockStorage()

	grpcApp := grpcapp.New(log, cfg.GRPC.Port, cfg.TokenTTL, hasher, storage, storage, storage)

	go func() {
		grpcApp.GRPCServer.MustRun()
	}()

	// Graceful Shutdown Setup
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Wait for SIGINT or SIGTERM
	<-ctx.Done()

	log.Info("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown http server", "error", err)
	}

	grpcApp.GRPCServer.Stop()

	log.Info("server stopped")
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
