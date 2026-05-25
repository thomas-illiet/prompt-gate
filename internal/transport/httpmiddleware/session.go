package middleware

import (
	"net/http"

	"promptgate/backend/internal/domain/auth"
)

// RequireSession returns a middleware that validates the session cookie and stores the user profile in the context.
func RequireSession(sessionStore *auth.SessionStore, cookieName string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				writeUnauthorized(w, "missing session")
				return
			}

			session, ok := sessionStore.Session(r.Context(), cookie.Value)
			if !ok {
				writeUnauthorized(w, "invalid session")
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.ContextWithUser(r.Context(), session.User)))
		})
	}
}
