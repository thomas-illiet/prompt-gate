package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"promptgate/backend/internal/platform/clientip"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before delegating to the underlying ResponseWriter.
func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// RequestLogger returns a middleware that logs each HTTP request with method, path, status, and duration.
func RequestLogger(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			writer := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(writer, r)

			attributes := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"status", writer.statusCode,
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"remote_addr", requestRemoteAddr(r),
				"user_agent", r.UserAgent(),
			}

			switch {
			case writer.statusCode >= http.StatusInternalServerError:
				logger.Error("http request completed", attributes...)
			case writer.statusCode >= http.StatusBadRequest:
				logger.Warn("http request completed", attributes...)
			default:
				logger.Info("http request completed", attributes...)
			}
		})
	}
}

// requestRemoteAddr returns the resolved context client IP or falls back to RemoteAddr.
func requestRemoteAddr(r *http.Request) string {
	if resolved := clientip.FromContext(r.Context()); resolved != "" {
		return resolved
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	return r.RemoteAddr
}
