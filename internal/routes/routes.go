package routes

import (
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/markosoft2000/auth/internal/http-server/handlers/health"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(log *slog.Logger, timeout int) *chi.Mux {
	r := chi.NewRouter()

	// Panic Recovery (Chi's built-in middleware)
	// This ensures the service doesn't crash on a panic and logs the stack trace.
	r.Use(middleware.Timeout(time.Duration(timeout) * time.Second))
	r.Use(middleware.Recoverer)

	r.Handle("/metrics", promhttp.Handler())
	r.Get("/health", health.New(log)) // Health Check endpoint

	return r
}
