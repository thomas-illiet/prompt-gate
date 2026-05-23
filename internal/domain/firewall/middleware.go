package firewall

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"promptgate/backend/internal/domain/auth"
)

// Middleware enforces firewall decisions from an in-memory snapshot.
func Middleware(snapshot *SnapshotStore, trustForwardHeaders bool, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := clientIP(r, trustForwardHeaders)
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
				if user, ok := auth.UserFromContext(r.Context()); ok && user.Type == auth.UserTypeService {
					args = append(args, "service_account_id", user.ID)
				}
				logger.Warn("request denied by firewall", args...)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "firewall_denied"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// allowsRequest chooses scoped service-account firewall rules when override is enabled.
func allowsRequest(snapshot *SnapshotStore, clientIP string, r *http.Request) (bool, *RuleResponse, error) {
	if user, ok := auth.UserFromContext(r.Context()); ok {
		return snapshot.AllowsUser(clientIP, user)
	}
	return snapshot.Allows(clientIP)
}

// clientIP extracts the client IP, optionally trusting forwarding headers.
func clientIP(r *http.Request, trustForwardHeaders bool) string {
	if trustForwardHeaders {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first, _, _ := strings.Cut(forwarded, ",")
			if first = strings.TrimSpace(first); first != "" {
				return first
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// writeJSON sends a JSON response for firewall middleware errors.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
