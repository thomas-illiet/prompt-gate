package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/tokens"
)

// handleCreateToken generates a new API token for the authenticated user.
func (s server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}

	var input tokens.CreateTokenRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request_body"})
		return
	}

	var trailingTokens any
	if err := decoder.Decode(&trailingTokens); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request_body"})
		return
	}

	resp, err := s.tokenService.CreateToken(r.Context(), user, input.Name, input.Description, input.ExpiresInDays)
	if err != nil {
		switch {
		case errors.Is(err, tokens.ErrInvalidName):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_token_name"})
			return
		case errors.Is(err, tokens.ErrInvalidTTL):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_token_ttl"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// handleListTokens returns all tokens owned by the authenticated user.
func (s server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}

	params := parseTokenListParams(r)
	result, err := s.tokenService.ListTokensPaged(r.Context(), user.ID, params)
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

// parseTokenListParams converts token query parameters into service filters.
func parseTokenListParams(r *http.Request) tokens.ListParams {
	query := r.URL.Query()
	page := parsePositiveInt(query.Get("page"), 1)
	pageSize := parsePositiveInt(query.Get("pageSize"), 10)
	if pageSize > 100 {
		pageSize = 100
	}
	sortBy := query.Get("sortBy")
	if sortBy == "" {
		sortBy = "createdAt"
	}
	sortDir := query.Get("sortDir")
	if sortDir == "" {
		sortDir = "desc"
	}
	return tokens.ListParams{
		Page:           page,
		PageSize:       pageSize,
		Search:         query.Get("search"),
		Status:         query.Get("status"),
		SortBy:         sortBy,
		SortDir:        sortDir,
		IncludeRevoked: true,
	}
}

// handleRevokeToken revokes one of the authenticated user's own tokens.
func (s server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}

	if err := s.tokenService.RevokeToken(r.Context(), user.ID, r.PathValue("id")); err != nil {
		if errors.Is(err, tokens.ErrTokenNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "token_not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
