package admin

import (
	"net/http"
	"time"
)

// HandleAdminDashboardQuotaRisk returns the SQL-backed quota-risk projection.
func (h *Handler) HandleAdminDashboardQuotaRisk(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscriptions service unavailable"})
		return
	}

	response, err := h.subscriptions.DashboardQuotaRisk(r.Context(), time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response)
}
