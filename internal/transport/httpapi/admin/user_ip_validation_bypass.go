package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
)

// HandleAdminSetUserIPValidationBypass sets or removes one human account's exception.
func (h *Handler) HandleAdminSetUserIPValidationBypass(w http.ResponseWriter, r *http.Request) {
	var input struct {
		BypassIPValidation *bool `json:"bypassIpValidation"`
	}
	if !decodeRequestBody(w, r, &input) {
		return
	}
	if input.BypassIPValidation == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request_body"})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user_not_found"})
		return
	}

	var actorUserID *string
	if actor, ok := auth.UserFromContext(r.Context()); ok {
		actorUserID = &actor.ID
	}
	err = h.users.SetIPValidationBypass(r.Context(), id.String(), *input.BypassIPValidation, actorUserID)
	switch {
	case errors.Is(err, users.ErrUserNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user_not_found"})
	case err != nil:
		slog.ErrorContext(r.Context(), "failed to set user IP validation bypass", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "ip_validation_bypass_update_failed"})
	default:
		writeJSON(w, http.StatusOK, input)
	}
}
