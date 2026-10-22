package admin

import (
	"net/http"
	"time"
)

// HandleAdminGroupStatistics returns usage statistics aggregated across the
// access group's current members.
func (h *Handler) HandleAdminGroupStatistics(w http.ResponseWriter, r *http.Request) {
	if h.groups == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "group service unavailable"})
		return
	}

	window, ok := h.adminDashboardRequest(w, r)
	if !ok {
		return
	}

	group, err := h.groups.GetGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		writeGroupError(w, err)
		return
	}

	memberIDs := make([]string, 0, len(group.Members))
	for _, member := range group.Members {
		memberIDs = append(memberIDs, member.ID)
	}
	response, err := h.proxy.DashboardOverviewForIdentities(r.Context(), memberIDs, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}
