package admin

import (
	"errors"
	"net/http"

	"promptgate/backend/internal/domain/proxy"
)

// HandleAdminListPrompts lists prompt history across all users.
func (h *Handler) HandleAdminListPrompts(w http.ResponseWriter, r *http.Request) {
	if h.proxy == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "proxy usage service unavailable",
		})
		return
	}

	query := parseListQuery(r, "createdAt", "desc")
	result, err := h.proxy.ListAdminPrompts(r.Context(), proxy.AdminPromptListParams{
		Page:     query.Page,
		PageSize: query.PageSize,
		Search:   r.URL.Query().Get("search"),
		SortBy:   query.SortBy,
		SortDir:  query.SortDir,
		UserID:   r.URL.Query().Get("userId"),
	})
	if err != nil {
		if errors.Is(err, proxy.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid_sort",
			})
			return
		}
		if errors.Is(err, proxy.ErrInvalidPagination) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid_pagination",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}
