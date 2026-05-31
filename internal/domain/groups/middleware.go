package groups

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"promptgate/backend/internal/domain/auth"
)

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

func requestProviderName(path string) string {
	path = strings.TrimPrefix(path, "/")
	providerName, _, _ := strings.Cut(path, "/")
	return strings.TrimSpace(providerName)
}

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

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
