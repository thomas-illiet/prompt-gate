package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
)

const (
	defaultUsersPage     = 1
	defaultUsersPageSize = 10
	maxUsersPageSize     = 100
)

// HandleAdminListUsers lists users with optional filtering by role, status, and search query.
func (h *Handler) HandleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	role := auth.AppRole(query.Get("role"))
	if role != "" && !role.IsValid() {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid_role",
		})
		return
	}

	status := query.Get("status")
	if status != "" && status != "active" && status != "inactive" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid_status",
		})
		return
	}

	page := parsePositiveInt(query.Get("page"), defaultUsersPage)
	pageSize := parsePositiveInt(query.Get("pageSize"), defaultUsersPageSize)
	if pageSize > maxUsersPageSize {
		pageSize = maxUsersPageSize
	}
	sortBy := query.Get("sortBy")
	if sortBy == "" {
		sortBy = "lastLoginAt"
	}
	sortDir := query.Get("sortDir")
	if sortDir == "" {
		sortDir = "desc"
	}

	result, err := h.users.ListUsers(r.Context(), users.ListParams{
		Page:     page,
		PageSize: pageSize,
		Search:   query.Get("search"),
		SortBy:   sortBy,
		SortDir:  sortDir,
		Type:     auth.UserTypeUser,
		Role:     role,
		Status:   status,
	})
	if err != nil {
		if errors.Is(err, users.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid_sort",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}
	if h.subscriptions != nil {
		if err := h.subscriptions.DecorateAdminUsers(r.Context(), result.Items); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleAdminGetUser returns a single user by ID.
func (h *Handler) HandleAdminGetUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.users.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, users.ErrUserNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "user_not_found",
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}
	if h.subscriptions != nil {
		items := []users.AdminUser{user}
		if err := h.subscriptions.DecorateAdminUsers(r.Context(), items); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		user = items[0]
	}

	writeJSON(w, http.StatusOK, user)
}

// HandleAdminUpdateUser updates a user's role and active status.
func (h *Handler) HandleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	var input users.UpdateUserInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid_request_body",
		})
		return
	}

	var trailingTokens any
	if err := decoder.Decode(&trailingTokens); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid_request_body",
		})
		return
	}

	user, err := h.users.UpdateUser(r.Context(), r.PathValue("id"), input)
	if err != nil {
		status := http.StatusInternalServerError
		payload := map[string]string{"error": err.Error()}

		switch {
		case errors.Is(err, users.ErrUserNotFound):
			status = http.StatusNotFound
			payload["error"] = "user_not_found"
		case errors.Is(err, users.ErrInvalidExpiration):
			status = http.StatusBadRequest
			payload["error"] = "invalid_expiration"
		case !input.Role.IsValid():
			status = http.StatusBadRequest
			payload["error"] = "invalid_role"
		}

		writeJSON(w, status, payload)
		return
	}
	if h.subscriptions != nil {
		items := []users.AdminUser{user}
		if err := h.subscriptions.DecorateAdminUsers(r.Context(), items); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		user = items[0]
	}

	writeJSON(w, http.StatusOK, user)
}

// HandleAdminUpdateUserNote updates a user's admin note.
func (h *Handler) HandleAdminUpdateUserNote(w http.ResponseWriter, r *http.Request) {
	var input users.UpdateAccountNoteInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	user, err := h.users.UpdateUserNote(r.Context(), r.PathValue("id"), input)
	if err != nil {
		status := http.StatusInternalServerError
		payload := map[string]string{"error": err.Error()}

		switch {
		case errors.Is(err, users.ErrUserNotFound):
			status = http.StatusNotFound
			payload["error"] = "user_not_found"
		case errors.Is(err, users.ErrInvalidNote):
			status = http.StatusBadRequest
			payload["error"] = "invalid_note"
		}

		writeJSON(w, status, payload)
		return
	}
	if h.subscriptions != nil {
		items := []users.AdminUser{user}
		if err := h.subscriptions.DecorateAdminUsers(r.Context(), items); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		user = items[0]
	}

	writeJSON(w, http.StatusOK, user)
}

// HandleAdminDeleteUser permanently deletes a user by ID.
func (h *Handler) HandleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	if err := h.users.DeleteUser(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, users.ErrUserNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "user_not_found",
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
