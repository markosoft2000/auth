package health

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

// New returns a handler function for the health check
func New(log *slog.Logger, db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Log that health was checked (optional, but good for debugging)
		log.Debug("health check hit")

		ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
		defer cancel()

		var (
			dbStatus = "ok"
			hasError = false
		)

		if err := db.Ping(ctx); err != nil {
			dbStatus = fmt.Sprintf("down: %v", err)
			hasError = true
		}

		response := map[string]any{
			"status": "up",
			"checks": map[string]string{
				"postgres": dbStatus,
			},
		}

		status := http.StatusOK
		if hasError {
			response["status"] = "down"
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(response)
	}
}
