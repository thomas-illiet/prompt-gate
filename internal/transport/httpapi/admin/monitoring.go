package admin

import (
	"errors"
	"net/http"

	"promptgate/backend/internal/domain/monitoring"
)

// HandleAdminListMonitoringServices lists configured HTTP/S monitoring services.
func (h *Handler) HandleAdminListMonitoringServices(w http.ResponseWriter, r *http.Request) {
	if !h.requireMonitoring(w) {
		return
	}

	query := parseListQuery(r, "name", "asc")
	services, err := h.monitoring.ListServicesPaged(r.Context(), monitoring.ListParams{
		Page:     query.Page,
		PageSize: query.PageSize,
		SortBy:   query.SortBy,
		SortDir:  query.SortDir,
	})
	if err != nil {
		if errors.Is(err, monitoring.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, services)
}

// HandleAdminGetMonitoringService returns one monitoring service by id.
func (h *Handler) HandleAdminGetMonitoringService(w http.ResponseWriter, r *http.Request) {
	if !h.requireMonitoring(w) {
		return
	}

	service, err := h.monitoring.GetService(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMonitoringError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, service)
}

// HandleAdminCreateMonitoringService creates an HTTP/S monitoring service.
func (h *Handler) HandleAdminCreateMonitoringService(w http.ResponseWriter, r *http.Request) {
	if !h.requireMonitoring(w) {
		return
	}

	var input monitoring.CreateServiceInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	service, err := h.monitoring.CreateService(r.Context(), input)
	if err != nil {
		writeMonitoringError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, service)
}

// HandleAdminUpdateMonitoringService patches a monitoring service.
func (h *Handler) HandleAdminUpdateMonitoringService(w http.ResponseWriter, r *http.Request) {
	if !h.requireMonitoring(w) {
		return
	}

	var input monitoring.UpdateServiceInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	service, err := h.monitoring.UpdateService(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeMonitoringError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, service)
}

// HandleAdminDeleteMonitoringService deletes a monitoring service.
func (h *Handler) HandleAdminDeleteMonitoringService(w http.ResponseWriter, r *http.Request) {
	if !h.requireMonitoring(w) {
		return
	}

	if err := h.monitoring.DeleteService(r.Context(), r.PathValue("id")); err != nil {
		writeMonitoringError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleAdminCheckMonitoringService runs one immediate monitoring check.
func (h *Handler) HandleAdminCheckMonitoringService(w http.ResponseWriter, r *http.Request) {
	if !h.requireMonitoring(w) {
		return
	}

	service, err := h.monitoring.CheckService(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMonitoringError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, service)
}

// requireMonitoring verifies that the admin monitoring service is wired.
func (h *Handler) requireMonitoring(w http.ResponseWriter) bool {
	if h.monitoring != nil {
		return true
	}

	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "monitoring service unavailable"})
	return false
}

// writeMonitoringError maps monitoring service errors to HTTP responses.
func writeMonitoringError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := err.Error()
	switch {
	case errors.Is(err, monitoring.ErrServiceNotFound):
		status = http.StatusNotFound
		code = "monitoring_service_not_found"
	case errors.Is(err, monitoring.ErrNameConflict):
		status = http.StatusConflict
		code = "name_conflict"
	case errors.Is(err, monitoring.ErrInvalidName):
		status = http.StatusBadRequest
		code = "invalid_name"
	case errors.Is(err, monitoring.ErrInvalidURL):
		status = http.StatusBadRequest
		code = "invalid_url"
	case errors.Is(err, monitoring.ErrInvalidStatus):
		status = http.StatusBadRequest
		code = "invalid_status_code"
	case errors.Is(err, monitoring.ErrInvalidInterval):
		status = http.StatusBadRequest
		code = "invalid_interval"
	}
	writeJSON(w, status, map[string]string{"error": code})
}
