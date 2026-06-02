package groups

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/provider"
)

// Middleware enforces group-based provider and model access from an in-memory snapshot.
func Middleware(snapshot *SnapshotStore, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
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
				next.ServeHTTP(w, r)
				return
			}

			model, err := requestModel(r)
			if err != nil {
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

// requestProviderName extracts the provider route prefix from the request path.
func requestProviderName(path string) string {
	path = strings.TrimPrefix(path, "/")
	providerName, _, _ := strings.Cut(path, "/")
	return strings.TrimSpace(providerName)
}

// requestProviderPath extracts the route suffix after the provider name.
func requestProviderPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	_, suffix, ok := strings.Cut(path, "/")
	if !ok {
		return "/"
	}
	return "/" + strings.TrimPrefix(suffix, "/")
}

// isModelBypassRoute reports whether a provider route can be checked without a request model.
func isModelBypassRoute(providerType provider.ProviderType, path string) bool {
	providerPath := requestProviderPath(path)
	switch providerType {
	case provider.ProviderTypeOpenAI:
		return routeExactOrSubtree(providerPath, "/v1/models") ||
			routeExactOrSubtree(providerPath, "/v1/conversations") ||
			routeSubtree(providerPath, "/v1/responses/")
	case provider.ProviderTypeOllama:
		return routeExactOrSubtree(providerPath, "/v1/models")
	case provider.ProviderTypeAnthropic:
		return routeExactOrSubtree(providerPath, "/v1/models") ||
			providerPath == "/v1/messages/count_tokens" ||
			routeSubtree(providerPath, "/api/event_logging/")
	default:
		return false
	}
}

// routeExactOrSubtree reports whether path matches route or a child route.
func routeExactOrSubtree(path, route string) bool {
	return path == route || strings.HasPrefix(path, route+"/")
}

// routeSubtree reports whether path is under routePrefix.
func routeSubtree(path, routePrefix string) bool {
	return strings.HasPrefix(path, routePrefix)
}

// requestModel reads and restores the request body while extracting the JSON model field.
func requestModel(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(raw)), nil
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", nil
	}

	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Model), nil
}

// writeJSON sends a JSON response for group middleware errors.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
