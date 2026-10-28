package admin

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/notifications"
)

type notificationPreviewRequest struct {
	Subject  string `json:"subject"`
	Markdown string `json:"markdown"`
}

// HandleAdminPreviewNotification renders safe text and HTML alternatives.
func (h *Handler) HandleAdminPreviewNotification(w http.ResponseWriter, r *http.Request) {
	if h.notifications == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notification_service_unavailable"})
		return
	}
	var input notificationPreviewRequest
	if !decodeRequestBody(w, r, &input) {
		return
	}
	textBody, htmlBody, err := h.notifications.Preview(input.Subject, input.Markdown)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"textBody": textBody, "htmlBody": htmlBody})
}

// HandleAdminResolveNotificationAudience previews resolved recipients and ignored accounts.
func (h *Handler) HandleAdminResolveNotificationAudience(w http.ResponseWriter, r *http.Request) {
	if h.notifications == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notification_service_unavailable"})
		return
	}
	var input notifications.AudienceRequest
	if !decodeRequestBody(w, r, &input) {
		return
	}
	result, err := h.notifications.ResolveAudience(r.Context(), input)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleAdminSendNotification enqueues a manual message for asynchronous delivery.
func (h *Handler) HandleAdminSendNotification(w http.ResponseWriter, r *http.Request) {
	if h.notifications == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notification_service_unavailable"})
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 512 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_idempotency_key"})
		return
	}
	var input notifications.ManualMessageRequest
	if !decodeRequestBody(w, r, &input) {
		return
	}
	var actorID *string
	if user, ok := auth.UserFromContext(r.Context()); ok {
		value := user.ID
		actorID = &value
	}
	message, audience, err := h.notifications.EnqueueManual(r.Context(), input, actorID, idempotencyKey)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"message": message, "audience": audience})
}

// HandleAdminListNotifications lists durable message history.
func (h *Handler) HandleAdminListNotifications(w http.ResponseWriter, r *http.Request) {
	if h.notifications == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notification_service_unavailable"})
		return
	}
	query := parseListQuery(r, "createdAt", "desc")
	result, err := h.notifications.ListMessages(r.Context(), notifications.ListParams{Page: query.Page, PageSize: query.PageSize, Search: r.URL.Query().Get("search"), Kind: notifications.Kind(r.URL.Query().Get("kind")), Status: notifications.DeliveryStatus(r.URL.Query().Get("status")), SortDir: query.SortDir})
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleAdminGetNotification returns stored content and delivery attempts.
func (h *Handler) HandleAdminGetNotification(w http.ResponseWriter, r *http.Request) {
	if h.notifications == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notification_service_unavailable"})
		return
	}
	result, err := h.notifications.GetMessage(r.Context(), r.PathValue("id"))
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleAdminRetryNotification resets only failed deliveries.
func (h *Handler) HandleAdminRetryNotification(w http.ResponseWriter, r *http.Request) {
	if h.notifications == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notification_service_unavailable"})
		return
	}
	count, err := h.notifications.RetryFailed(r.Context(), r.PathValue("id"), time.Now().UTC())
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"retried": count})
}

func writeNotificationError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "notification_failed"
	switch {
	case errors.Is(err, notifications.ErrMessageNotFound):
		status, code = http.StatusNotFound, "notification_not_found"
	case errors.Is(err, notifications.ErrInvalidAddress):
		status, code = http.StatusBadRequest, "invalid_email_address"
	case errors.Is(err, notifications.ErrInvalidSubject):
		status, code = http.StatusBadRequest, "invalid_email_subject"
	case errors.Is(err, notifications.ErrInvalidBody):
		status, code = http.StatusBadRequest, "invalid_email_body"
	case errors.Is(err, notifications.ErrInvalidAudience):
		status, code = http.StatusBadRequest, "invalid_email_audience"
	case errors.Is(err, notifications.ErrInvalidSort):
		status, code = http.StatusBadRequest, "invalid_sort"
	case strings.Contains(err.Error(), "notifications disabled"):
		status, code = http.StatusServiceUnavailable, "notifications_disabled"
	}
	writeJSON(w, status, map[string]string{"error": code})
}
