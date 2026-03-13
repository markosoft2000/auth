package health

import (
	"log/slog"
	"net/http"
)

// New returns a handler function for the health check
func New(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Log that health was checked (optional, but good for debugging)
		log.Debug("health check hit")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up"}`))
	}
}
