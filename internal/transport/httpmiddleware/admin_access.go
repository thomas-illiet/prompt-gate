package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"promptgate/backend/internal/domain/auth"
)

const AdminAPIKeyHeader = "X-Admin-API-Key"

// RequireAdminAccess accepts either the configured administration API key or an authenticated admin session.
func RequireAdminAccess(sessionStore *auth.SessionStore, cookieName string, adminAPIKey string) Middleware {
	configuredKeyHash := sha256.Sum256([]byte(adminAPIKey))
	adminAPIKeyConfigured := adminAPIKey != ""
	headerName := http.CanonicalHeaderKey(AdminAPIKeyHeader)

	return func(next http.Handler) http.Handler {
		sessionHandler := Chain(
			next,
			RequireSession(sessionStore, cookieName),
			RequireAppAccess(),
			RequireRoles(auth.RoleAdmin),
		)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			values, present := r.Header[headerName]
			if !present {
				sessionHandler.ServeHTTP(w, r)
				return
			}

			if !adminAPIKeyConfigured || len(values) != 1 || values[0] == "" {
				writeUnauthorized(w, "invalid_admin_api_key")
				return
			}

			providedKeyHash := sha256.Sum256([]byte(values[0]))
			if subtle.ConstantTimeCompare(providedKeyHash[:], configuredKeyHash[:]) != 1 {
				writeUnauthorized(w, "invalid_admin_api_key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
