package cli

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"gorm.io/gorm"

	"promptgate/backend/internal/platform/health"
	"promptgate/backend/internal/platform/redisstore"
)

func startDiagnostics(ctx context.Context, address string, db *gorm.DB, redis *redisstore.Store, progress *health.Progress) (func(), error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /live", func(w http.ResponseWriter, r *http.Request) {
		if !progress.Healthy() {
			http.Error(w, "background job stalled", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if ctx.Err() != nil || !progress.Healthy() || db.WithContext(checkCtx).Exec("SELECT 1").Error != nil || redis.Client().Ping(checkCtx).Err() != nil {
			http.Error(w, "background service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("background health server failed", "error", err)
		}
	}()
	return func() { _ = server.Close() }, nil
}
