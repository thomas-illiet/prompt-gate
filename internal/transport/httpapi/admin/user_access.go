package admin

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/modellab"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/requestprivacy"
)

// HandleAdminUserQuota returns the live subscription quota for a human user.
func (h *Handler) HandleAdminUserQuota(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	userID, ok := h.targetHumanUserID(w, r)
	if !ok {
		return
	}
	if h.quotaRedis == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscription quota service unavailable"})
		return
	}
	status, err := h.quotaRedis.ReadCurrentQuota(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load_subscription_quota_failed"})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// HandleAdminUserModels returns the Model Lab catalog allowed for a human user.
func (h *Handler) HandleAdminUserModels(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(requestprivacy.MarkMetadataOnly(r.Context()))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Pragma", "no-cache")
	userID, ok := h.targetHumanUserID(w, r)
	if !ok {
		return
	}
	if h.modelLab == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "model_lab_unavailable"})
		return
	}
	if h.groups == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "model_lab_catalog_unavailable"})
		return
	}
	access, err := h.groups.UserAccess(r.Context(), userID)
	if err != nil {
		if errors.Is(err, groups.ErrUserNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user_not_found"})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "model_lab_catalog_unavailable"})
		return
	}
	catalog, err := h.modelLab.Catalog(r.Context(), modellab.AccessFilterForGroups(access))
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "model_lab_catalog_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (h *Handler) targetHumanUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	if h.users == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user service unavailable"})
		return "", false
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user_not_found"})
		return "", false
	}
	user, err := h.users.UserByID(r.Context(), id.String())
	if err != nil && !errors.Is(err, users.ErrUserNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load_user_failed"})
		return "", false
	}
	if errors.Is(err, users.ErrUserNotFound) || user.Type != auth.UserTypeUser {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user_not_found"})
		return "", false
	}
	return user.ID, true
}
