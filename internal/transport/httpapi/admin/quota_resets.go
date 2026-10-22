package admin

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/subscriptions"
)

// HandleAdminGrantQuotaReset offers exactly one reset credit to a human account.
func (h *Handler) HandleAdminGrantQuotaReset(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "quota_reset_service_unavailable"})
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_idempotency_key"})
		return
	}

	actor := subscriptions.QuotaResetActor{Type: subscriptions.QuotaResetActorAdminAPIKey}
	if user, ok := auth.UserFromContext(r.Context()); ok {
		actorID := user.ID
		actor = subscriptions.QuotaResetActor{
			Type:   subscriptions.QuotaResetActorAdminSession,
			UserID: &actorID,
		}
	}

	summary, err := h.subscriptions.GrantQuotaResetCredit(
		r.Context(),
		r.PathValue("id"),
		actor,
		idempotencyKey,
		time.Now().UTC(),
	)
	if err != nil {
		writeAdminQuotaResetError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func writeAdminQuotaResetError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "quota_reset_failed"
	switch {
	case errors.Is(err, subscriptions.ErrInvalidQuotaResetIdempotencyKey):
		status = http.StatusBadRequest
		code = "missing_idempotency_key"
	case errors.Is(err, subscriptions.ErrQuotaResetUserNotFound):
		status = http.StatusNotFound
		code = "user_not_found"
	case errors.Is(err, subscriptions.ErrQuotaResetBalanceFull):
		status = http.StatusConflict
		code = "quota_reset_balance_full"
	case errors.Is(err, subscriptions.ErrQuotaResetUnavailable):
		status = http.StatusConflict
		code = "quota_reset_credit_unavailable"
	case errors.Is(err, subscriptions.ErrQuotaResetNotApplicable):
		status = http.StatusConflict
		code = "quota_reset_not_applicable"
	case errors.Is(err, subscriptions.ErrQuotaResetPending):
		status = http.StatusConflict
		code = "quota_reset_pending"
	}
	writeJSON(w, status, map[string]string{"error": code})
}
