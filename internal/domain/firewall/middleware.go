package firewall

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/clientip"
)

// Middleware enforces firewall decisions from an in-memory snapshot.
func Middleware(snapshot *SnapshotStore, trustForwardHeaders bool, logger *slog.Logger) func(http.Handler) http.Handler {
	return MiddlewareWithOptions(snapshot, clientip.Options{TrustForwardHeaders: trustForwardHeaders}, logger)
}

// MiddlewareWithOptions enforces firewall decisions using the configured client IP policy.
func MiddlewareWithOptions(snapshot *SnapshotStore, options clientip.Options, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := clientip.FromContext(r.Context())
			if clientIP == "" {
				clientIP = clientip.ResolveWithOptions(r, options)
			}
			allowed, rule, err := allowsRequest(snapshot, clientIP, r)
			if err != nil {
				logger.Warn("firewall check failed", "error", err, "client_ip", clientIP)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "firewall_denied"})
				return
			}
			if !allowed {
				args := []any{"client_ip", clientIP}
				if rule != nil {
					args = append(args, "rule_id", rule.ID)
				}
				if user, ok := auth.UserFromContext(r.Context()); ok {
					switch user.Type {
					case auth.UserTypeService:
						args = append(args, "service_account_id", user.ID)
					case auth.UserTypeUser:
						args = append(args, "user_id", user.ID)
					}
				}
				logger.Warn("request denied by firewall", args...)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "firewall_denied"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// allowsRequest chooses scoped firewall rules when override is enabled.
func allowsRequest(snapshot *SnapshotStore, clientIP string, r *http.Request) (bool, *RuleResponse, error) {
	if user, ok := auth.UserFromContext(r.Context()); ok {
		return snapshot.AllowsUser(clientIP, user)
	}
	return snapshot.Allows(clientIP)
}

// writeJSON sends a JSON response for firewall middleware errors.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
