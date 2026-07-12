package groups

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/proxylimits"
)

type MiddlewareOptions struct {
	MaxBufferedRequestBytes int64
}

// Middleware enforces group-based provider and model access from an in-memory snapshot.
func Middleware(snapshot *SnapshotStore, logger *slog.Logger) func(http.Handler) http.Handler {
	return MiddlewareWithOptions(snapshot, logger, MiddlewareOptions{})
}

// MiddlewareWithOptions enforces group-based provider and model access with explicit buffering limits.
func MiddlewareWithOptions(snapshot *SnapshotStore, logger *slog.Logger, opts MiddlewareOptions) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if opts.MaxBufferedRequestBytes <= 0 {
		opts.MaxBufferedRequestBytes = proxylimits.DefaultMaxBufferedRequestBytes
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing_authenticated_user"})
				return
			}
			providerName := requestProviderName(r.URL.Path)
			if providerName == "" || snapshot == nil || !snapshot.KnownProvider(providerName) {
				logger.Warn("group access denied for unknown provider", "provider", providerName, "user_id", user.ID)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_access_denied"})
				return
			}
			providerType, _ := snapshot.ProviderType(providerName)
			if isModelBypassRoute(providerType, r.URL.Path) {
				if !snapshot.AllowsProvider(user.ID, providerName) {
					logger.Warn("group access denied for provider route", "provider", providerName, "path", r.URL.Path, "user_id", user.ID)
					writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_access_denied"})
					return
				}
				if isFilterableModelListRoute(providerType, r.URL.Path, r.Method) {
					filterModelListResponse(w, r, next, snapshot, user.ID, providerName)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			model, err := requestModel(r, opts.MaxBufferedRequestBytes)
			if err != nil {
				if errors.Is(err, proxylimits.ErrExceeded) {
					logger.Warn("group access denied for oversized request body", "provider", providerName, "user_id", user.ID, "limit_bytes", opts.MaxBufferedRequestBytes)
					writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request_body_too_large"})
					return
				}
				logger.Warn("group access denied", "provider", providerName, "model", model, "user_id", user.ID)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_access_denied"})
				return
			}
			if model == "" {
				logger.Warn("group access denied for request without model", "provider", providerName, "user_id", user.ID)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_model_required"})
				return
			}
			if !snapshot.Allows(user.ID, providerName, model) {
				logger.Warn("group access denied", "provider", providerName, "model", model, "user_id", user.ID)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_access_denied"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeJSON sends a JSON response for group middleware errors.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
