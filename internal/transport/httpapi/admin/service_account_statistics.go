package admin

import (
	"net/http"
	"time"
)

// HandleAdminServiceAccountStatistics returns aggregated usage statistics for one service account.
func (h *Handler) HandleAdminServiceAccountStatistics(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user service unavailable"})
		return
	}

	window, ok := h.adminDashboardRequest(w, r)
	if !ok {
		return
	}

	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	response, err := h.proxy.DashboardOverview(r.Context(), account.ID, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}
