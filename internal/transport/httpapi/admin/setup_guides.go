package admin

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"promptgate/backend/internal/domain/setupguide"
)

func (h *Handler) HandleAdminListSetupGuides(w http.ResponseWriter, r *http.Request) {
	if h.setupGuides == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "setup guide service unavailable"})
		return
	}
	items, err := h.setupGuides.List(r.Context(), false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h *Handler) HandleAdminGetSetupGuide(w http.ResponseWriter, r *http.Request) {
	id, ok := setupGuideID(w, r)
	if !ok {
		return
	}
	item, err := h.setupGuides.Get(r.Context(), id)
	if writeSetupGuideError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h *Handler) HandleAdminCreateSetupGuide(w http.ResponseWriter, r *http.Request) {
	var input setupguide.Input
	if !decodeRequestBody(w, r, &input) {
		return
	}
	item, err := h.setupGuides.Create(r.Context(), input)
	if writeSetupGuideError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h *Handler) HandleAdminUpdateSetupGuide(w http.ResponseWriter, r *http.Request) {
	id, ok := setupGuideID(w, r)
	if !ok {
		return
	}
	var input setupguide.Input
	if !decodeRequestBody(w, r, &input) {
		return
	}
	item, err := h.setupGuides.Update(r.Context(), id, input)
	if writeSetupGuideError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h *Handler) HandleAdminDeleteSetupGuide(w http.ResponseWriter, r *http.Request) {
	id, ok := setupGuideID(w, r)
	if !ok {
		return
	}
	if writeSetupGuideError(w, h.setupGuides.Delete(r.Context(), id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) HandleAdminReorderSetupGuides(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if !decodeRequestBody(w, r, &body) {
		return
	}
	if err := h.setupGuides.Reorder(r.Context(), body.IDs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	items, err := h.setupGuides.List(r.Context(), false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h *Handler) HandleAdminValidateSetupGuide(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Template string `json:"template"`
	}
	if !decodeRequestBody(w, r, &body) {
		return
	}
	if err := setupguide.ValidateTemplate(body.Template); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}
func setupGuideID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid setup guide id"})
		return uuid.Nil, false
	}
	return id, true
}
func writeSetupGuideError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, setupguide.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "setup_guide_not_found"})
	case errors.Is(err, gorm.ErrDuplicatedKey):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "setup_guide_identifier_conflict"})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return true
}
