package grpcapp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	authgrpc "github.com/markosoft2000/auth/internal/grpc/auth"
	"github.com/markosoft2000/auth/internal/grpc/interceptors/validator"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type App struct {
	log               *slog.Logger
	gRPCServer        *grpc.Server
	port              int
	healthSrv         *health.Server
	dbPinger          Pinger
	pubsubPinger      Pinger
	healthCheckCancel context.CancelFunc // To cancel the HealthCheck goroutine
}

func New(
	log *slog.Logger,
	port int,
	authService authgrpc.Auth,
	dbPinger Pinger,
	pubsubPinger Pinger,
) *App {
	loggingOpts := []logging.Option{
		logging.WithLogOnEvents(
			logging.PayloadReceived, logging.PayloadSent,
		),
	}

	recoveryOpts := []recovery.Option{
		recovery.WithRecoveryHandler(func(p any) (err error) {
			log.Error("Recovered from panic", slog.Any("panic", p))

			return status.Errorf(codes.Internal, "internal error")
		}),
	}

	gRPCServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		recovery.UnaryServerInterceptor(recoveryOpts...),
		logging.UnaryServerInterceptor(InterceptorLogger(log), loggingOpts...),
		validator.UnaryServerInterceptor(log),
	))

	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(gRPCServer, healthSrv)

	authgrpc.Register(gRPCServer, authService)

	return &App{
		log:               log,
		gRPCServer:        gRPCServer,
		port:              port,
		healthSrv:         healthSrv,
		dbPinger:          dbPinger,
		pubsubPinger:      pubsubPinger,
		healthCheckCancel: nil, // Will be set in Run
	}
}

func InterceptorLogger(l *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	const op = "grpcapp.Run"

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.log.Info("grpc server started", slog.String("addr", l.Addr().String()))

	// Create a cancellable context for the HealthCheck goroutine
	healthCtx, healthCancel := context.WithCancel(context.Background())
	a.healthCheckCancel = healthCancel
	go a.HealthCheck(healthCtx)

	if err := a.gRPCServer.Serve(l); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *App) Stop() {
	const op = "grpcapp.Stop"

	// Signal the HealthCheck goroutine to stop
	if a.healthCheckCancel != nil {
		a.healthCheckCancel()
	}

	a.log.With(slog.String("op", op)).Info("stopping grpc server", slog.Int("port", a.port))
	a.gRPCServer.GracefulStop()
}

func (a *App) HealthCheck(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check Postgres status
			dbStatus := grpc_health_v1.HealthCheckResponse_SERVING
			if err := a.dbPinger.Ping(ctx); err != nil {
				a.log.Error("health check failed", slog.String("service", "postgres"), slog.Any("error", err))
				dbStatus = grpc_health_v1.HealthCheckResponse_NOT_SERVING
			}

			// Check pubsub status
			pubsubStatus := grpc_health_v1.HealthCheckResponse_SERVING
			if err := a.pubsubPinger.Ping(ctx); err != nil {
				a.log.Error("health check failed", slog.String("service", "pubsub"), slog.Any("error", err))
				pubsubStatus = grpc_health_v1.HealthCheckResponse_NOT_SERVING
			}

			// Update health server
			status := dbStatus
			if pubsubStatus == grpc_health_v1.HealthCheckResponse_NOT_SERVING {
				status = pubsubStatus
			}
			a.healthSrv.SetServingStatus("", status)
			a.healthSrv.SetServingStatus("auth.Auth", status)

		case <-ctx.Done():
			a.log.Debug("HealthCheck goroutine stopped due to context cancellation")
			return
		}
	}
}
