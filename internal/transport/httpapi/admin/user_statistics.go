package admin

import (
	"errors"
	"net/http"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
)

// HandleAdminUserStatistics returns aggregated usage statistics for one human user.
func (h *Handler) HandleAdminUserStatistics(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user service unavailable"})
		return
	}

	window, ok := h.adminDashboardRequest(w, r)
	if !ok {
		return
	}

	user, err := h.users.UserByID(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, users.ErrUserNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user_not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if user.Type != auth.UserTypeUser {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user_not_found"})
		return
	}

	response, err := h.proxy.DashboardOverview(r.Context(), user.ID, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}
