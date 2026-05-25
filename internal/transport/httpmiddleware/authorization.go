package middleware

import (
	"encoding/json"
	"net/http"

	"promptgate/backend/internal/domain/auth"
)

// RequireAppAccess returns a middleware that rejects inactive users or those with role "none".
func RequireAppAccess() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				writeAuthorizationFailure(w, http.StatusInternalServerError, "missing_authenticated_user")
				return
			}

			if !user.IsActive {
				writeAuthorizationFailure(w, http.StatusForbidden, "account_inactive")
				return
			}

			if user.Role == auth.RoleNone {
				writeAuthorizationFailure(w, http.StatusForbidden, "account_role_none")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRoles returns a middleware that allows access only to users with one of the specified roles.
func RequireRoles(roles ...auth.AppRole) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				writeAuthorizationFailure(w, http.StatusInternalServerError, "missing_authenticated_user")
				return
			}

			if !user.IsActive {
				writeAuthorizationFailure(w, http.StatusForbidden, "account_inactive")
				return
			}

			if user.Role == auth.RoleNone {
				writeAuthorizationFailure(w, http.StatusForbidden, "account_role_none")
				return
			}

			for _, role := range roles {
				if user.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeAuthorizationFailure(w, http.StatusForbidden, "insufficient_role")
		})
	}
}

// writeAuthorizationFailure writes a JSON error response with the given HTTP status and error code.
func writeAuthorizationFailure(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": code,
	})
}
