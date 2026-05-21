package auth

import (
	"encoding/json"
	"net/http"

	coderbridge "github.com/coder/aibridge"
)

// ActorMiddleware injects the authenticated user as an promptgate actor.
func ActorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing_authenticated_user"})
			return
		}
		metadata := coderbridge.Metadata{
			"email":             user.Email,
			"name":              user.Name,
			"preferredUsername": user.PreferredUsername,
			"role":              string(user.Role),
		}
		next.ServeHTTP(w, r.WithContext(coderbridge.AsActor(r.Context(), user.ID, metadata)))
	})
}

// writeJSON sends a JSON response for auth middleware errors.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
