package tokens

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
)

type MiddlewareOptions struct {
	TokenService *Service
	UserResolver UserResolver
	Cache        AuthCache
	Logger       *slog.Logger
}

// MiddlewareWithOptions validates proxy auth from an API token.
func MiddlewareWithOptions(opts MiddlewareOptions) func(http.Handler) http.Handler {
	cache := opts.Cache
	if cache == nil {
		cache = NoopAuthCache{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, authenticated := auth.PrincipalFromContext(r.Context())
			if !authenticated {
				rawToken, ok := bearerToken(r.Header.Get("Authorization"))
				if ok {
					tokenHash := sha256hex(rawToken)
					var expiresAt time.Time
					var generation int64
					var cacheErr error
					principal, generation, authenticated, cacheErr = cache.Get(r.Context(), tokenHash)
					if cacheErr != nil {
						principal = auth.Principal{}
						authenticated = false
					}
					if !authenticated {
						var err error
						principal, expiresAt, err = opts.TokenService.ValidatePrincipalWithExpiry(r.Context(), rawToken, opts.UserResolver)
						if err != nil {
							writeAuthError(w, err)
							return
						}
						if cacheErr == nil {
							cache.Set(r.Context(), generation, tokenHash, principal, time.Until(expiresAt))
						}
						authenticated = true
					}
				}
			}

			if !authenticated {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing_auth_credentials"})
				return
			}

			r.Header.Del("Authorization")
			r.Header.Del("X-Api-Key")
			next.ServeHTTP(w, r.WithContext(auth.ContextWithPrincipal(r.Context(), principal)))
		})
	}
}

// bearerToken extracts a bearer token from an Authorization header.
func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

// writeAuthError maps token validation errors to HTTP responses.
func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrAccountInactive):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "account_inactive"})
	case errors.Is(err, ErrInsufficientRole):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient_role"})
	default:
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
	}
}

// writeJSON sends a JSON response for token middleware errors.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
