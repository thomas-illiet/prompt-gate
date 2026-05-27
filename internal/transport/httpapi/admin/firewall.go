package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"promptgate/backend/internal/domain/firewall"
)

// HandleAdminListFirewallRules returns all firewall rules ordered by priority.
func (h *Handler) HandleAdminListFirewallRules(w http.ResponseWriter, r *http.Request) {
	query := parseListQuery(r, "priority", "asc")
	rules, err := h.firewall.ListRulesPaged(r.Context(), firewall.ListParams{
		Page:     query.Page,
		PageSize: query.PageSize,
		SortBy:   query.SortBy,
		SortDir:  query.SortDir,
	})
	if err != nil {
		if errors.Is(err, firewall.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, rules)
}

// HandleAdminGetFirewallRule returns a single firewall rule by ID.
func (h *Handler) HandleAdminGetFirewallRule(w http.ResponseWriter, r *http.Request) {
	rule, err := h.firewall.GetRule(r.Context(), r.PathValue("id"))
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// HandleAdminCreateFirewallRule creates a firewall rule.
func (h *Handler) HandleAdminCreateFirewallRule(w http.ResponseWriter, r *http.Request) {
	var input firewall.CreateRuleInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	rule, err := h.firewall.CreateRule(r.Context(), input)
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}

// HandleAdminUpdateFirewallRule partially updates a firewall rule.
func (h *Handler) HandleAdminUpdateFirewallRule(w http.ResponseWriter, r *http.Request) {
	var input firewall.UpdateRuleInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	rule, err := h.firewall.UpdateRule(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// HandleAdminMoveFirewallRulePriority moves a firewall rule priority by one slot.
func (h *Handler) HandleAdminMoveFirewallRulePriority(w http.ResponseWriter, r *http.Request) {
	var input firewall.MovePriorityInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	rule, err := h.firewall.MovePriority(r.Context(), r.PathValue("id"), input.Direction)
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// HandleAdminSimulateFirewallRule evaluates one client IP against enabled firewall rules.
func (h *Handler) HandleAdminSimulateFirewallRule(w http.ResponseWriter, r *http.Request) {
	var input simulateFirewallInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	allowed, matchedRule, err := h.firewall.Allows(r.Context(), input.ClientIP)
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, simulateFirewallResponse{
		Allowed:     allowed,
		MatchedRule: matchedRule,
	})
}

// HandleAdminDeleteFirewallRule permanently deletes a firewall rule.
func (h *Handler) HandleAdminDeleteFirewallRule(w http.ResponseWriter, r *http.Request) {
	if err := h.firewall.DeleteRule(r.Context(), r.PathValue("id")); err != nil {
		writeFirewallError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type simulateFirewallInput struct {
	ClientIP string `json:"clientIp"`
}

type simulateFirewallResponse struct {
	Allowed     bool                   `json:"allowed"`
	MatchedRule *firewall.RuleResponse `json:"matchedRule"`
}

// decodeRequestBody decodes a JSON admin payload and writes a client error on failure.
func decodeRequestBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request_body"})
		return false
	}

	var trailingTokens any
	if err := decoder.Decode(&trailingTokens); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request_body"})
		return false
	}

	return true
}

// writeFirewallError maps firewall service errors to HTTP responses.
func writeFirewallError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := err.Error()

	switch {
	case errors.Is(err, firewall.ErrRuleNotFound):
		status = http.StatusNotFound
		code = "firewall_rule_not_found"
	case errors.Is(err, firewall.ErrPriorityConflict):
		status = http.StatusConflict
		code = "priority_conflict"
	case errors.Is(err, firewall.ErrInvalidAddress):
		status = http.StatusBadRequest
		code = "invalid_ipv4_address"
	case errors.Is(err, firewall.ErrInvalidAction):
		status = http.StatusBadRequest
		code = "invalid_action"
	case errors.Is(err, firewall.ErrPriorityOutOfRange):
		status = http.StatusBadRequest
		code = "priority_out_of_range"
	case errors.Is(err, firewall.ErrInvalidDirection):
		status = http.StatusBadRequest
		code = "invalid_direction"
	}

	writeJSON(w, status, map[string]string{"error": code})
}
