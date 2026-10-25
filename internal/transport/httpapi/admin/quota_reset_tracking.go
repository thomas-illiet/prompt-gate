package admin

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/subscriptions"
)

// HandleAdminListQuotaStates returns the durable quota projections used by the
// operational reset dashboard.
func (h *Handler) HandleAdminListQuotaStates(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "quota_reset_service_unavailable"})
		return
	}
	query := r.URL.Query()
	result, err := h.subscriptions.AdminQuotaStates(r.Context(), subscriptions.AdminQuotaStateParams{
		View:        query.Get("view"),
		AccountType: auth.UserType(query.Get("accountType")),
		Search:      query.Get("search"),
		Page:        parsePositiveInt(query.Get("page"), 1),
		PageSize:    min(parsePositiveInt(query.Get("pageSize"), 20), 100),
		SortBy:      query.Get("sortBy"),
		SortDir:     query.Get("sortDir"),
	}, time.Now().UTC())
	if err != nil {
		writeAdminQuotaTrackingError(w, err, "load_quota_states_failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleAdminListQuotaResetEvents returns the durable reset-credit ledger.
func (h *Handler) HandleAdminListQuotaResetEvents(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "quota_reset_service_unavailable"})
		return
	}
	query := r.URL.Query()
	result, err := h.subscriptions.AdminQuotaResetEvents(r.Context(), subscriptions.AdminQuotaResetEventParams{
		Window:   query.Get("window"),
		Type:     subscriptions.QuotaResetEventType(query.Get("type")),
		Status:   subscriptions.QuotaResetEventStatus(query.Get("status")),
		Search:   query.Get("search"),
		Page:     parsePositiveInt(query.Get("page"), 1),
		PageSize: min(parsePositiveInt(query.Get("pageSize"), 20), 100),
		SortBy:   query.Get("sortBy"),
		SortDir:  query.Get("sortDir"),
	}, time.Now().UTC())
	if err != nil {
		writeAdminQuotaTrackingError(w, err, "load_quota_reset_events_failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeAdminQuotaTrackingError(w http.ResponseWriter, err error, fallback string) {
	status := http.StatusInternalServerError
	code := fallback
	switch {
	case errors.Is(err, subscriptions.ErrInvalidQuotaStateView):
		status = http.StatusBadRequest
		code = "invalid_quota_view"
	case errors.Is(err, subscriptions.ErrInvalidQuotaAccountType):
		status = http.StatusBadRequest
		code = "invalid_account_type"
	case errors.Is(err, subscriptions.ErrInvalidQuotaResetWindow):
		status = http.StatusBadRequest
		code = "invalid_usage_window"
	case errors.Is(err, subscriptions.ErrInvalidQuotaResetEventType):
		status = http.StatusBadRequest
		code = "invalid_quota_reset_type"
	case errors.Is(err, subscriptions.ErrInvalidQuotaResetEventStatus):
		status = http.StatusBadRequest
		code = "invalid_quota_reset_status"
	case errors.Is(err, subscriptions.ErrInvalidSort):
		status = http.StatusBadRequest
		code = "invalid_sort"
	}
	writeJSON(w, status, map[string]string{"error": strings.TrimSpace(code)})
}
