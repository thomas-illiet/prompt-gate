package policy

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/clientip"
)

func Middleware(evaluator *Evaluator, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusInternalServerError, "missing_authenticated_user")
				return
			}
			input := Input{
				Identity: IdentityInput{
					ID: principal.User.ID, Type: string(principal.User.Type), Role: string(principal.User.Role),
					CredentialID: principal.CredentialID, CredentialName: principal.CredentialName,
				},
				Request: RequestInput{
					ClientIP: clientip.FromContext(r.Context()), Method: r.Method, Path: r.URL.Path, Host: r.Host,
				},
			}
			decision, err := evaluator.Evaluate(r.Context(), input)
			if err != nil {
				logger.Error("policy decision unavailable", "error", err)
				writeJSON(w, http.StatusServiceUnavailable, "policy_unavailable")
				return
			}
			if !decision.Allow {
				logger.Warn("request denied by policy", "reason", decision.Reason)
				writeJSON(w, http.StatusForbidden, "policy_denied")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
