package admin

import (
	"errors"
	"net/http"

	"promptgate/backend/internal/domain/faq"
)

func (h *Handler) HandleAdminListFAQ(w http.ResponseWriter, r *http.Request) {
	if !h.requireFAQ(w) {
		return
	}
	query := parseListQuery(r, "position", "asc")
	result, err := h.faq.List(r.Context(), faq.ListParams{Page: query.Page, PageSize: query.PageSize, SortBy: query.SortBy, SortDir: query.SortDir})
	if err != nil {
		writeFAQError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) HandleAdminGetFAQ(w http.ResponseWriter, r *http.Request) {
	if !h.requireFAQ(w) {
		return
	}
	entry, err := h.faq.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeFAQError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *Handler) HandleAdminCreateFAQ(w http.ResponseWriter, r *http.Request) {
	if !h.requireFAQ(w) {
		return
	}
	var input faq.Input
	if !decodeRequestBody(w, r, &input) {
		return
	}
	entry, err := h.faq.Create(r.Context(), input)
	if err != nil {
		writeFAQError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (h *Handler) HandleAdminUpdateFAQ(w http.ResponseWriter, r *http.Request) {
	if !h.requireFAQ(w) {
		return
	}
	var input faq.UpdateInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	entry, err := h.faq.Update(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeFAQError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *Handler) HandleAdminMoveFAQ(w http.ResponseWriter, r *http.Request) {
	if !h.requireFAQ(w) {
		return
	}
	var input faq.PositionInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	entry, err := h.faq.Move(r.Context(), r.PathValue("id"), input.Position)
	if err != nil {
		writeFAQError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *Handler) HandleAdminDeleteFAQ(w http.ResponseWriter, r *http.Request) {
	if !h.requireFAQ(w) {
		return
	}
	if err := h.faq.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeFAQError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HandleAdminPreviewFAQ(w http.ResponseWriter, r *http.Request) {
	if !h.requireFAQ(w) {
		return
	}
	var input faq.PreviewInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	html, err := h.faq.Render(input.Markdown)
	if err != nil {
		writeFAQError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"renderedHtml": html})
}

func (h *Handler) requireFAQ(w http.ResponseWriter) bool {
	if h.faq != nil {
		return true
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "faq service unavailable"})
	return false
}

func writeFAQError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, faq.ErrNotFound):
		status, code = http.StatusNotFound, "faq_not_found"
	case errors.Is(err, faq.ErrInvalidID):
		status, code = http.StatusBadRequest, "invalid_id"
	case errors.Is(err, faq.ErrQuestion):
		status, code = http.StatusBadRequest, "question_required"
	case errors.Is(err, faq.ErrQuestionLength):
		status, code = http.StatusBadRequest, "question_too_long"
	case errors.Is(err, faq.ErrAnswer):
		status, code = http.StatusBadRequest, "answer_required"
	case errors.Is(err, faq.ErrInvalidPosition):
		status, code = http.StatusBadRequest, "invalid_position"
	case errors.Is(err, faq.ErrInvalidSort):
		status, code = http.StatusBadRequest, "invalid_sort"
	}
	writeJSON(w, status, map[string]string{"error": code})
}
