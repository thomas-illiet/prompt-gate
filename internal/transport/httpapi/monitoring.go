package httpapi

import "net/http"

// handleMonitoringStatus returns currently degraded services for app users.
func (s server) handleMonitoringStatus(w http.ResponseWriter, r *http.Request) {
	if s.monitoring == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "monitoring service unavailable"})
		return
	}

	status, err := s.monitoring.CurrentStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}
