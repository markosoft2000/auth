package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	grpcapp "github.com/markosoft2000/auth/internal/app/grpc"
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

type App struct {
	cfg *config.Config
	log *slog.Logger

	grpcServer *grpcapp.App
	httpServer *http.Server

	pgStorage    *postgres.Storage
	redisStorage *redis.Storage
	pubsub       *kafka.PubSub
}

func New(
	ctx context.Context,
	log *slog.Logger,
	cfg *config.Config,
) *App {
	hasher := argon2.New(
		cfg.Hasher.Memory,
		cfg.Hasher.Iterations,
		cfg.Hasher.Parallelism,
		cfg.Hasher.SaltLength,
		cfg.Hasher.KeyLength,
	)

	cipher := gcmCipher.New(cfg.MasterSecret)

	pgStorage := newPGStorage(log, &cfg.Postgres)

	var appStorage auth.AppManager = pgStorage
	var tokenStorage auth.TokenManager = pgStorage

	var redisStorage *redis.Storage

	if cfg.Caching.Enabled {
		// Redis cluster ready solution
		var err error
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

	pubsub := newKafkaPubSub(ctx, log, &cfg.Kafka)

	authCfg := auth.AuthCfg{
		TokenTTL:               cfg.TokenTTL,
		RefreshTokenTTL:        cfg.RefreshTokenTTL,
		ReissueRefreshTokenTTL: cfg.ReissueRefreshTokenTTL,
	}

	storage := auth.Storage{
		UserSaver:    pgStorage,
		UserProvider: pgStorage,
		AppManager:   appStorage,
		TokenManager: tokenStorage,
	}

	authService := auth.New(log, authCfg, hasher, cipher, storage, pubsub)
	grpcApp := grpcapp.New(log, cfg.GRPC.Port, authService, pgStorage, pubsub)

	// HTTP Server Configuration
	r := routes.NewRouter(log, cfg.HTTPServer.Timeout, pgStorage)
	srv := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.HTTPServer.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTPServer.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.HTTPServer.IdleTimeout) * time.Second,
	}

	return &App{
		cfg: cfg,
		log: log,

		grpcServer: grpcApp,
		httpServer: srv,

		pgStorage:    pgStorage,
		redisStorage: redisStorage,
		pubsub:       pubsub,
	}
}

func (app *App) MustRun() {
	go func() {
		app.grpcServer.MustRun()
	}()

	go func() {
		app.log.Info("http server starting", slog.String("addr", app.cfg.HTTPServer.Address))

		if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.log.Error("http server failed", slog.Any("error", err))
		}
	}()
}

func (app *App) Stop(ctx context.Context) {
	app.log.Info("shutting down gracefully...")

	start := time.Now()

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := app.httpServer.Shutdown(shutdownCtx); err != nil {
		app.log.Error("forced shutdown http server", "error", err)
	}

	app.grpcServer.Stop()

	app.pubsub.Stop()
	if app.cfg.Caching.Enabled {
		app.redisStorage.Stop()
	}
	app.pgStorage.Stop()

	app.log.Info("server stopped in ", slog.Duration("duration (ms)", time.Duration(time.Since(start).Milliseconds())))
}

func newPGStorage(log *slog.Logger, cfg *config.PostgresConfig) *postgres.Storage {
	pgMasterCfg := &postgres.Config{
		Host:     cfg.Master.Host,
		Port:     cfg.Master.Port,
		User:     cfg.Master.User,
		Password: cfg.Master.Password,
		Database: cfg.Master.Database,
		SSLMode:  cfg.Master.SSLMode,
	}
	pgReplicaCfg := &postgres.Config{
		Host:     cfg.Replica.Host,
		Port:     cfg.Replica.Port,
		User:     cfg.Replica.User,
		Password: cfg.Replica.Password,
		Database: cfg.Replica.Database,
		SSLMode:  cfg.Replica.SSLMode,
	}
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

	return pgStorage
}

func newKafkaPubSub(ctx context.Context, log *slog.Logger, cfg *config.KafkaConfig) *kafka.PubSub {
	pubsub, err := kafka.New(ctx, log, kafka.Config{
		BootstrapServers:      cfg.BootstrapServers,
		ClientID:              cfg.ClientID,
		Topic:                 cfg.Topic,
		BatchNumMessages:      cfg.BatchNumMessages,
		LingerMs:              cfg.LingerMs,
		CompressionType:       cfg.CompressionType,
		Acks:                  cfg.Acks,
		EnableIdempotence:     cfg.EnableIdempotence,
		Retries:               cfg.Retries,
		RetryBackoffMs:        cfg.RetryBackoffMs,
		MessageTimeoutMs:      cfg.MessageTimeoutMs,
		SocketKeepaliveEnable: cfg.SocketKeepaliveEnable,
		QueueBufferingMaxMsgs: cfg.QueueBufferingMaxMsgs,

		ProducerMaxRetries:   cfg.ProducerMaxRetries,
		ProducerRetryBackoff: cfg.ProducerRetryBackoff,
	})
	if err != nil {
		log.Error("failed to init kafka", slog.Any("error", err))
		os.Exit(1)
	}

	return pubsub
}
