package admin

import (
	"errors"
	"net/http"

	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/users"
)

func (h *Handler) HandleAdminListSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscriptions service unavailable"})
		return
	}
	params := parseListQuery(r, "name", "asc")
	result, err := h.subscriptions.ListPlansPaged(r.Context(), subscriptions.PlanListParams{
		Page:     params.Page,
		PageSize: params.PageSize,
		SortBy:   params.SortBy,
		SortDir:  params.SortDir,
	})
	if err != nil {
		if errors.Is(err, subscriptions.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) HandleAdminCreateSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscriptions service unavailable"})
		return
	}
	var input subscriptions.PlanInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	plan, err := h.subscriptions.CreatePlan(r.Context(), input)
	if writeSubscriptionPlanError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (h *Handler) HandleAdminGetSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscriptions service unavailable"})
		return
	}
	plan, err := h.subscriptions.GetPlan(r.Context(), r.PathValue("id"))
	if writeSubscriptionPlanError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) HandleAdminUpdateSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscriptions service unavailable"})
		return
	}
	var input subscriptions.PlanInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	plan, err := h.subscriptions.UpdatePlan(r.Context(), r.PathValue("id"), input)
	if writeSubscriptionPlanError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) HandleAdminSetDefaultSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscriptions service unavailable"})
		return
	}
	plan, err := h.subscriptions.SetDefaultPlan(r.Context(), r.PathValue("id"))
	if writeSubscriptionPlanError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) HandleAdminDeleteSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscriptions service unavailable"})
		return
	}
	if err := h.subscriptions.DeletePlan(r.Context(), r.PathValue("id")); err != nil {
		if writeSubscriptionPlanError(w, err) {
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HandleAdminAssignUserSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscriptions service unavailable"})
		return
	}
	var input subscriptions.AssignPlanInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	user, err := h.subscriptions.AssignUserPlan(r.Context(), r.PathValue("id"), input.PlanID)
	if writeSubscriptionAssignmentError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) HandleAdminAssignServiceAccountSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	if h.subscriptions == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "subscriptions service unavailable"})
		return
	}
	var input subscriptions.AssignPlanInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	account, err := h.subscriptions.AssignServiceAccountPlan(r.Context(), r.PathValue("id"), input.PlanID)
	if writeSubscriptionAssignmentError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func writeSubscriptionPlanError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	status := http.StatusInternalServerError
	code := err.Error()
	switch {
	case errors.Is(err, subscriptions.ErrPlanNotFound):
		status = http.StatusNotFound
		code = "subscription_plan_not_found"
	case errors.Is(err, subscriptions.ErrInvalidPlan):
		status = http.StatusBadRequest
		code = "invalid_subscription_plan"
	case errors.Is(err, subscriptions.ErrDefaultPlanDelete):
		status = http.StatusBadRequest
		code = "default_plan_delete_denied"
	case errors.Is(err, subscriptions.ErrPlanAssigned):
		status = http.StatusConflict
		code = "subscription_plan_assigned"
	}
	writeJSON(w, status, map[string]string{"error": code})
	return true
}

func writeSubscriptionAssignmentError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	status := http.StatusInternalServerError
	code := err.Error()
	switch {
	case errors.Is(err, users.ErrUserNotFound):
		status = http.StatusNotFound
		code = "user_not_found"
	case errors.Is(err, subscriptions.ErrInvalidAssignment):
		status = http.StatusBadRequest
		code = "invalid_subscription_assignment"
	}
	writeJSON(w, status, map[string]string{"error": code})
	return true
}
