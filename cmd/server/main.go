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
	gcmCipher "github.com/markosoft2000/auth/internal/lib/crypt"
	"github.com/markosoft2000/auth/internal/lib/hasher/argon2"
	"github.com/markosoft2000/auth/internal/pubsub/kafka"
	"github.com/markosoft2000/auth/internal/routes"
	"github.com/markosoft2000/auth/internal/service/auth"
	"github.com/markosoft2000/auth/internal/service/auth/cache"
	"github.com/markosoft2000/auth/internal/storage/postgres"
	"github.com/markosoft2000/auth/internal/storage/redis"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	// GRPC Server Configuration
	hasher := argon2.New(
		cfg.Hasher.Memory,
		cfg.Hasher.Iterations,
		cfg.Hasher.Parallelism,
		cfg.Hasher.SaltLength,
		cfg.Hasher.KeyLength,
	)

	cipher := gcmCipher.New(cfg.MasterSecret)

	pgMasterCfg := &postgres.Config{
		Host:     cfg.Postgres.Host,
		Port:     cfg.Postgres.Port,
		User:     cfg.Postgres.User,
		Password: cfg.Postgres.Password,
		Database: cfg.Postgres.Database,
		SSLMode:  cfg.Postgres.SSLMode,
	}
	pgReplicaCfg := pgMasterCfg
	// Create a cluster without single point of failure (SPOF).
	// * HAProxy with dual listeners - listens on two different TCP ports:
	// Port 5432 (Write): Routes exclusively to the current Master node.
	// Port 5433 (Read): Load balances across all healthy Replica nodes.
	// * PgBouncer configured in transaction mode.
	// Schema:
	// App -> Virtual IP (Keepalived) -> Active HAProxy (+Patroni API) ->
	// -> Master's PgBouncer (Port 6432) -> PgBouncer Local PostgreSQL (Port 5432)
	pgStorage, err := postgres.New(pgMasterCfg, pgReplicaCfg)
	if err != nil {
		log.Error("failed to init storage", slog.Any("error", err))
		os.Exit(1)
	}

	var appStorage auth.AppManager = pgStorage
	var tokenStorage auth.TokenManager = pgStorage

	var redisStorage *redis.Storage

	if cfg.Caching.Enabled {
		// Redis cluster ready solution
		redisStorage, err = redis.New(redis.Config{
			Addresses:        cfg.Redis.Addresses,
			OperationTimeout: cfg.Redis.OperationTimeout,
			AppTTL:           cfg.Caching.AppTTL,
			RefreshTokenTTL:  cfg.Caching.RefreshTokenTTL,
		})
		if err != nil {
			log.Error("failed to init redis", slog.Any("error", err))
			os.Exit(1)
		}

		appStorage = cache.NewAppCache(log, redisStorage, appStorage)
		tokenStorage = cache.NewTokenCache(log, redisStorage, tokenStorage)
	}

	pubsub, err := kafka.New(ctx, log, kafka.Config{
		BootstrapServers:      cfg.Kafka.BootstrapServers,
		ClientID:              cfg.Kafka.ClientID,
		Topic:                 cfg.Kafka.Topic,
		BatchNumMessages:      cfg.Kafka.BatchNumMessages,
		LingerMs:              cfg.Kafka.LingerMs,
		CompressionType:       cfg.Kafka.CompressionType,
		Acks:                  cfg.Kafka.Acks,
		EnableIdempotence:     cfg.Kafka.EnableIdempotence,
		Retries:               cfg.Kafka.Retries,
		RetryBackoffMs:        cfg.Kafka.RetryBackoffMs,
		MessageTimeoutMs:      cfg.Kafka.MessageTimeoutMs,
		SocketKeepaliveEnable: cfg.Kafka.SocketKeepaliveEnable,
		QueueBufferingMaxMsgs: cfg.Kafka.QueueBufferingMaxMsgs,

		ProducerMaxRetries:   cfg.Kafka.ProducerMaxRetries,
		ProducerRetryBackoff: cfg.Kafka.ProducerRetryBackoff,
	})
	if err != nil {
		log.Error("failed to init kafka", slog.Any("error", err))
		os.Exit(1)
	}

	grpcApp := grpcapp.New(
		log,
		cfg.GRPC.Port,
		auth.AuthCfg{
			TokenTTL:               cfg.TokenTTL,
			RefreshTokenTTL:        cfg.RefreshTokenTTL,
			ReissueRefreshTokenTTL: cfg.ReissueRefreshTokenTTL,
		},
		hasher,
		cipher,
		auth.Storage{
			UserSaver:    pgStorage,
			UserProvider: pgStorage,
			AppManager:   appStorage,
			TokenManager: tokenStorage,
		},
		pgStorage,
		pubsub,
		pubsub,
	)
	go func() {
		grpcApp.GRPCServer.MustRun()
	}()

	// HTTP Server Configuration
	r := routes.NewRouter(log, cfg.HTTPServer.Timeout, pgStorage)
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
		}
	}()

	// Graceful Shutdown Setup
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Wait for SIGINT or SIGTERM
	<-sigCtx.Done()

	log.Info("shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown http server", "error", err)
	}

	grpcApp.GRPCServer.Stop()
	pubsub.Stop()
	if cfg.Caching.Enabled {
		redisStorage.Stop()
	}
	pgStorage.Stop()

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
