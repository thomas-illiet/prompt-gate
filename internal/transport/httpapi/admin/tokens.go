package admin

import (
	"errors"
	"net/http"

	"promptgate/backend/internal/domain/tokens"
)

// HandleAdminListUserTokens returns all tokens for a given user (admin only).
func (h *Handler) HandleAdminListUserTokens(w http.ResponseWriter, r *http.Request) {
	query := parseListQuery(r, "createdAt", "desc")
	result, err := h.tokens.ListTokensPaged(r.Context(), r.PathValue("id"), tokens.ListParams{
		Page:           query.Page,
		PageSize:       query.PageSize,
		SortBy:         query.SortBy,
		SortDir:        query.SortDir,
		IncludeRevoked: true,
	})
	if err != nil {
		if errors.Is(err, tokens.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleAdminRevokeToken revokes any token by ID regardless of owner (admin only).
func (h *Handler) HandleAdminRevokeToken(w http.ResponseWriter, r *http.Request) {
	if err := h.tokens.AdminRevokeToken(r.Context(), r.PathValue("tokenId")); err != nil {
		if errors.Is(err, tokens.ErrTokenNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "token_not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
