package policy

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/requestprivacy"
)

// DenialRecorder persists authenticated policy denials without coupling policy
// evaluation to the proxy usage-event implementation.
type DenialRecorder interface {
	RecordPolicyDenial(ctx context.Context, accountID, clientIP, reason string, deniedAt time.Time) error
}

func Middleware(evaluator *Evaluator, snapshot *groups.SnapshotStore, denialRecorder DenialRecorder, logger *slog.Logger) func(http.Handler) http.Handler {
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
			groupNames := []string{}
			if snapshot != nil {
				groupNames = snapshot.GroupNames(principal.User.ID)
			}
			providerName, providerType, providerPath := policyRequestTarget(r.URL.Path, snapshot)
			input := Input{
				Identity: IdentityInput{
					ID: principal.User.ID, Type: string(principal.User.Type), Role: string(principal.User.Role),
					Groups:       groupNames,
					CredentialID: principal.CredentialID, CredentialName: principal.CredentialName,
				},
				Request: RequestInput{
					ClientIP: clientip.FromContext(r.Context()), Method: r.Method, Path: r.URL.Path, Host: r.Host,
					Provider: providerName, ProviderType: providerType, ProviderPath: providerPath,
				},
			}
			decision, err := evaluator.Evaluate(r.Context(), input)
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
					if err := denialRecorder.RecordPolicyDenial(r.Context(), principal.User.ID, input.Request.ClientIP, decision.Reason, time.Now().UTC()); err != nil {
						logger.Error("failed to record policy denial", "error", err)
					}
				}
				writeJSON(w, http.StatusForbidden, "policy_denied")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func policyRequestTarget(path string, snapshot *groups.SnapshotStore) (name, providerType, relativePath string) {
	trimmed := strings.TrimPrefix(path, "/")
	name, suffix, found := strings.Cut(trimmed, "/")
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", "/"
	}
	relativePath = "/"
	if found && suffix != "" {
		relativePath += strings.TrimPrefix(suffix, "/")
	}
	if snapshot != nil {
		if value, ok := snapshot.ProviderType(name); ok {
			providerType = string(value)
		}
	}
	return name, providerType, relativePath
}

func writeJSON(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
