package admin

import (
	"errors"
	"net/http"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
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

// HandleAdminCreateUserToken creates a token for any human user.
func (h *Handler) HandleAdminCreateUserToken(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	user, err := h.users.UserByID(r.Context(), userID)
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

	var input tokens.CreateTokenRequest
	if !decodeRequestBody(w, r, &input) {
		return
	}

	created, err := h.tokens.CreateToken(r.Context(), user, input.Name, input.Description, input.ExpiresInDays)
	if err != nil {
		writeTokenCreationError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, created)
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
