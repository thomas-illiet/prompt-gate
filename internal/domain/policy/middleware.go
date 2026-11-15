package policy

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/modellab"
	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/requestprivacy"
)

// DenialRecorder persists authenticated policy denials without coupling policy
// evaluation to the proxy usage-event implementation.
type DenialRecorder interface {
	RecordPolicyDenial(ctx context.Context, accountID, clientIP, reason string, deniedAt time.Time) error
}

// UserBypassResolver reads the current IP validation exception for an authenticated account.
type UserBypassResolver interface {
	CanBypassIPValidation(ctx context.Context, id string) (bool, error)
}

func Middleware(evaluator *Evaluator, denialRecorder DenialRecorder, userBypass UserBypassResolver, logger *slog.Logger) func(http.Handler) http.Handler {
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
			if modellab.HasValidatedTicket(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
			if userBypass != nil && principal.User.Type == auth.UserTypeUser {
				bypass, err := userBypass.CanBypassIPValidation(r.Context(), principal.User.ID)
				if err != nil {
					evaluator.record(r.Context(), "user_bypass_unavailable", false)
					logger.Warn("user IP validation bypass lookup failed")
				} else if bypass {
					evaluator.record(r.Context(), "user_bypass", true)
					next.ServeHTTP(w, r)
					return
				}
			}
			ip := clientip.FromContext(r.Context())
			decision, err := evaluator.Evaluate(r.Context(), ip)
			if err != nil {
				if requestprivacy.IsMetadataOnly(r.Context()) {
					logger.Error("policy decision unavailable", "privacy_mode", "metadata_only")
				} else {
					logger.Error("policy decision unavailable", "error", err)
				}
				writeJSON(w, http.StatusServiceUnavailable, "policy_unavailable")
				return
			}
			if !decision.Allow {
				if requestprivacy.IsMetadataOnly(r.Context()) {
					logger.Warn("request denied by policy", "privacy_mode", "metadata_only")
				} else {
					logger.Warn("request denied by policy", "reason", decision.Reason)
				}
				if denialRecorder != nil {
					if err := denialRecorder.RecordPolicyDenial(r.Context(), principal.User.ID, ip, decision.Reason, time.Now().UTC()); err != nil {
						logger.Error("failed to record policy denial", "error", err)
					}
				}
				writeJSON(w, http.StatusForbidden, DeniedReason)
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
