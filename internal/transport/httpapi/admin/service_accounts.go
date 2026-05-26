package admin

import (
	"errors"
	"net/http"

	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
)

// HandleAdminListServiceAccounts lists service accounts.
func (h *Handler) HandleAdminListServiceAccounts(w http.ResponseWriter, r *http.Request) {
	query := parseListQuery(r, "createdAt", "desc")
	list, err := h.users.ListServiceAccountsPaged(r.Context(), users.ServiceAccountListParams{
		Page:     query.Page,
		PageSize: query.PageSize,
		SortBy:   query.SortBy,
		SortDir:  query.SortDir,
	})
	if err != nil {
		if errors.Is(err, users.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// HandleAdminCreateServiceAccount creates a service account.
func (h *Handler) HandleAdminCreateServiceAccount(w http.ResponseWriter, r *http.Request) {
	var input users.ServiceAccountInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	account, err := h.users.CreateServiceAccount(r.Context(), input)
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, account)
}

// HandleAdminGetServiceAccount returns one service account by ID.
func (h *Handler) HandleAdminGetServiceAccount(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.GetServiceAccount(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, account)
}

// HandleAdminUpdateServiceAccount updates a service account.
func (h *Handler) HandleAdminUpdateServiceAccount(w http.ResponseWriter, r *http.Request) {
	var input users.ServiceAccountInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	account, err := h.users.UpdateServiceAccount(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, account)
}

// HandleAdminDeleteServiceAccount deletes a service account.
func (h *Handler) HandleAdminDeleteServiceAccount(w http.ResponseWriter, r *http.Request) {
	if err := h.users.DeleteServiceAccount(r.Context(), r.PathValue("id")); err != nil {
		writeServiceAccountError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleAdminListServiceAccountTokens lists tokens for a service account.
func (h *Handler) HandleAdminListServiceAccountTokens(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	includeRevoked := r.URL.Query().Get("includeRevoked") == "true"
	query := parseListQuery(r, "createdAt", "desc")
	list, err := h.tokens.ListTokensPaged(r.Context(), account.ID, tokens.ListParams{
		Page:           query.Page,
		PageSize:       query.PageSize,
		SortBy:         query.SortBy,
		SortDir:        query.SortDir,
		IncludeRevoked: includeRevoked,
	})
	if err != nil {
		if errors.Is(err, tokens.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// HandleAdminCreateServiceAccountToken creates a token for a service account.
func (h *Handler) HandleAdminCreateServiceAccountToken(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	var input tokens.CreateTokenRequest
	if !decodeRequestBody(w, r, &input) {
		return
	}

	token, err := h.tokens.CreateToken(r.Context(), account, input.Name, input.Description, input.ExpiresInDays)
	if err != nil {
		writeTokenCreationError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, token)
}

// HandleAdminRevokeServiceAccountToken revokes a service account token.
func (h *Handler) HandleAdminRevokeServiceAccountToken(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	if err := h.tokens.RevokeToken(r.Context(), account.ID, r.PathValue("tokenId")); err != nil {
		if errors.Is(err, tokens.ErrTokenNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "token_not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleAdminListServiceAccountFirewallRules lists scoped firewall rules for a service account.
func (h *Handler) HandleAdminListServiceAccountFirewallRules(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	query := parseListQuery(r, "priority", "asc")
	list, err := h.firewall.ListServiceAccountRulesPaged(r.Context(), account.ID, firewall.ListParams{
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

	writeJSON(w, http.StatusOK, list)
}

// HandleAdminCreateServiceAccountFirewallRule creates a scoped firewall rule.
func (h *Handler) HandleAdminCreateServiceAccountFirewallRule(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	var input firewall.CreateRuleInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	rule, err := h.firewall.CreateServiceAccountRule(r.Context(), account.ID, input)
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}

// HandleAdminGetServiceAccountFirewallRule returns one scoped firewall rule.
func (h *Handler) HandleAdminGetServiceAccountFirewallRule(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	rule, err := h.firewall.GetServiceAccountRule(r.Context(), account.ID, r.PathValue("ruleId"))
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// HandleAdminUpdateServiceAccountFirewallRule updates one scoped firewall rule.
func (h *Handler) HandleAdminUpdateServiceAccountFirewallRule(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	var input firewall.UpdateRuleInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	rule, err := h.firewall.UpdateServiceAccountRule(r.Context(), account.ID, r.PathValue("ruleId"), input)
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// HandleAdminMoveServiceAccountFirewallRulePriority moves one scoped firewall rule priority.
func (h *Handler) HandleAdminMoveServiceAccountFirewallRulePriority(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	var input firewall.MovePriorityInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	rule, err := h.firewall.MoveServiceAccountPriority(r.Context(), account.ID, r.PathValue("ruleId"), input.Direction)
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// HandleAdminSimulateServiceAccountFirewallRule evaluates one client IP against scoped rules.
func (h *Handler) HandleAdminSimulateServiceAccountFirewallRule(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	var input simulateFirewallInput
	if !decodeRequestBody(w, r, &input) {
		return
	}

	allowed, matchedRule, err := h.firewall.ServiceAccountAllows(r.Context(), account.ID, input.ClientIP)
	if err != nil {
		writeFirewallError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, simulateFirewallResponse{
		Allowed:     allowed,
		MatchedRule: matchedRule,
	})
}

// HandleAdminDeleteServiceAccountFirewallRule deletes one scoped firewall rule.
func (h *Handler) HandleAdminDeleteServiceAccountFirewallRule(w http.ResponseWriter, r *http.Request) {
	account, err := h.users.ServiceAccountProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceAccountError(w, err)
		return
	}

	if err := h.firewall.DeleteServiceAccountRule(r.Context(), account.ID, r.PathValue("ruleId")); err != nil {
		writeFirewallError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeServiceAccountError maps service account domain errors to API responses.
func writeServiceAccountError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := err.Error()

	switch {
	case errors.Is(err, users.ErrUserNotFound):
		status = http.StatusNotFound
		code = "service_account_not_found"
	case errors.Is(err, users.ErrInvalidServiceAccountIdentifier):
		status = http.StatusBadRequest
		code = "invalid_identifier"
	case errors.Is(err, users.ErrInvalidServiceAccountName):
		status = http.StatusBadRequest
		code = "invalid_name"
	case errors.Is(err, users.ErrServiceAccountConflict):
		status = http.StatusConflict
		code = "identifier_conflict"
	}

	writeJSON(w, status, map[string]string{"error": code})
}

// writeTokenCreationError maps token creation errors to service account API responses.
func writeTokenCreationError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := err.Error()

	switch {
	case errors.Is(err, tokens.ErrInvalidName):
		status = http.StatusBadRequest
		code = "invalid_token_name"
	case errors.Is(err, tokens.ErrInvalidTTL):
		status = http.StatusBadRequest
		code = "invalid_token_ttl"
	}

	writeJSON(w, status, map[string]string{"error": code})
}
