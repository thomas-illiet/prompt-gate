package httpapi

import (
	"net/http"
	"time"

	"promptgate/backend/internal/domain/auth"
)

func (s server) handleCurrentUserQuota(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}
	if s.quotaRedis == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscription quota service unavailable"})
		return
	}
	status, err := s.quotaRedis.CurrentQuota(r.Context(), user.ID, time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load_subscription_quota_failed"})
		return
	}
	writeJSON(w, http.StatusOK, status)
}
