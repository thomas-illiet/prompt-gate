package admin

import (
	"errors"
	"net/http"
	"strings"

	"promptgate/backend/internal/domain/provider"
)

// HandleAdminListProviders lists configured LLM providers.
func (h *Handler) HandleAdminListProviders(w http.ResponseWriter, r *http.Request) {
	query := parseListQuery(r, "name", "asc")
	providers, err := h.providers.ListProvidersPaged(r.Context(), provider.ListParams{
		Page:     query.Page,
		PageSize: query.PageSize,
		SortBy:   query.SortBy,
		SortDir:  query.SortDir,
	})
	if err != nil {
		if errors.Is(err, provider.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, providers)
}

// HandleAdminProviderModelCatalog returns best-effort upstream model lists for providers.
func (h *Handler) HandleAdminProviderModelCatalog(w http.ResponseWriter, r *http.Request) {
	if h.providers == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "provider service unavailable"})
		return
	}
	catalog, err := h.providers.ModelCatalog(r.Context(), providerIDsFromQuery(r))
	if err != nil {
		if errors.Is(err, provider.ErrProviderNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider_not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

// HandleAdminGetProvider returns one LLM provider by id.
func (h *Handler) HandleAdminGetProvider(w http.ResponseWriter, r *http.Request) {
	record, err := h.providers.GetProvider(r.Context(), r.PathValue("id"))
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

// HandleAdminCreateProvider creates an LLM provider configuration.
func (h *Handler) HandleAdminCreateProvider(w http.ResponseWriter, r *http.Request) {
	var input provider.CreateProviderInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	record, err := h.providers.CreateProvider(r.Context(), input)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

// HandleAdminUpdateProvider patches an LLM provider configuration.
func (h *Handler) HandleAdminUpdateProvider(w http.ResponseWriter, r *http.Request) {
	var input provider.UpdateProviderInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	record, err := h.providers.UpdateProvider(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

// HandleAdminDeleteProvider deletes an LLM provider configuration.
func (h *Handler) HandleAdminDeleteProvider(w http.ResponseWriter, r *http.Request) {
	if err := h.providers.DeleteProvider(r.Context(), r.PathValue("id")); err != nil {
		writeProviderError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeProviderError maps provider service errors to HTTP responses.
func writeProviderError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := err.Error()
	switch {
	case errors.Is(err, provider.ErrProviderNotFound):
		status = http.StatusNotFound
		code = "provider_not_found"
	case errors.Is(err, provider.ErrNameConflict):
		status = http.StatusConflict
		code = "name_conflict"
	case errors.Is(err, provider.ErrInvalidName):
		status = http.StatusBadRequest
		code = "invalid_name"
	case errors.Is(err, provider.ErrInvalidType):
		status = http.StatusBadRequest
		code = "invalid_type"
	case errors.Is(err, provider.ErrInvalidURL):
		status = http.StatusBadRequest
		code = "invalid_url"
	}
	writeJSON(w, status, map[string]string{"error": code})
}

func providerIDsFromQuery(r *http.Request) []string {
	query := r.URL.Query()
	values := append([]string{}, query["providerId"]...)
	for _, raw := range query["providerIds"] {
		values = append(values, strings.Split(raw, ",")...)
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
