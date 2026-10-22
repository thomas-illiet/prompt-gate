package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/subscriptions"
)

const idempotencyKeyHeader = "Idempotency-Key"

func (s server) handleCurrentUserQuotaResetSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}
	if s.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "quota_reset_service_unavailable"})
		return
	}

	summary, err := s.subscriptions.QuotaResetSummary(r.Context(), user.ID, time.Now().UTC())
	if err != nil {
		writeQuotaResetError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s server) handleCurrentUserQuotaResetUse(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}
	if s.subscriptions == nil || s.quotaRedis == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "quota_reset_service_unavailable"})
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get(idempotencyKeyHeader))
	if idempotencyKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_idempotency_key"})
		return
	}

	result, err := s.subscriptions.UseQuotaReset(
		r.Context(),
		s.quotaRedis,
		user.ID,
		idempotencyKey,
		time.Now().UTC(),
	)
	if err != nil {
		writeQuotaResetError(w, err)
		return
	}
	status := http.StatusOK
	if result.Status == subscriptions.QuotaResetUsePending {
		status = http.StatusAccepted
	}
	writeJSON(w, status, result)
}

func (s server) handleCurrentUserQuotaResetHistory(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}
	if s.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "quota_reset_service_unavailable"})
		return
	}

	history, err := s.subscriptions.ListQuotaResetHistory(r.Context(), user.ID, subscriptions.QuotaResetHistoryParams{
		Page:     positiveQueryInt(r, "page", 1, 0),
		PageSize: positiveQueryInt(r, "pageSize", 20, 100),
	})
	if err != nil {
		writeQuotaResetError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func positiveQueryInt(r *http.Request, name string, fallback, maximum int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || value <= 0 {
		return fallback
	}
	if maximum > 0 && value > maximum {
		return maximum
	}
	return value
}

func writeQuotaResetError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "quota_reset_failed"
	switch {
	case errors.Is(err, subscriptions.ErrInvalidQuotaResetIdempotencyKey):
		status = http.StatusBadRequest
		code = "missing_idempotency_key"
	case errors.Is(err, subscriptions.ErrQuotaResetUserNotFound):
		status = http.StatusNotFound
		code = "user_not_found"
	case errors.Is(err, subscriptions.ErrQuotaResetUnavailable):
		status = http.StatusConflict
		code = "quota_reset_credit_unavailable"
	case errors.Is(err, subscriptions.ErrQuotaResetNotApplicable):
		status = http.StatusConflict
		code = "quota_reset_not_applicable"
	case errors.Is(err, subscriptions.ErrQuotaResetBalanceFull):
		status = http.StatusConflict
		code = "quota_reset_balance_full"
	case errors.Is(err, subscriptions.ErrQuotaResetPending):
		status = http.StatusConflict
		code = "quota_reset_pending"
	}
	writeJSON(w, status, map[string]string{"error": code})
}
