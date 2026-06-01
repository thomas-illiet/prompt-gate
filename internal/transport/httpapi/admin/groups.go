package admin

import (
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/provider"
)

// HandleAdminListGroups lists access groups with pagination, search, and sorting.
func (h *Handler) HandleAdminListGroups(w http.ResponseWriter, r *http.Request) {
	query := parseListQuery(r, "name", "asc")
	result, err := h.groups.ListGroupsPaged(r.Context(), groups.ListParams{
		Page:     query.Page,
		PageSize: query.PageSize,
		Search:   r.URL.Query().Get("search"),
		SortBy:   query.SortBy,
		SortDir:  query.SortDir,
	})
	if err != nil {
		if errors.Is(err, groups.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleAdminGetGroup returns a single access group by ID.
func (h *Handler) HandleAdminGetGroup(w http.ResponseWriter, r *http.Request) {
	group, err := h.groups.GetGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, group)
}

// HandleAdminCreateGroup creates a new access group.
func (h *Handler) HandleAdminCreateGroup(w http.ResponseWriter, r *http.Request) {
	var input groups.CreateGroupInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	group, err := h.groups.CreateGroup(r.Context(), input)
	if err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

// HandleAdminUpdateGroup updates an existing access group.
func (h *Handler) HandleAdminUpdateGroup(w http.ResponseWriter, r *http.Request) {
	var input groups.UpdateGroupInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	group, err := h.groups.UpdateGroup(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, group)
}

// HandleAdminDeleteGroup deletes an access group by ID.
func (h *Handler) HandleAdminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if err := h.groups.DeleteGroup(r.Context(), r.PathValue("id")); err != nil {
		writeGroupError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleAdminAddGroupMember adds a user to an access group.
func (h *Handler) HandleAdminAddGroupMember(w http.ResponseWriter, r *http.Request) {
	if err := h.groups.AddMember(r.Context(), r.PathValue("id"), r.PathValue("userId")); err != nil {
		writeGroupError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleAdminRemoveGroupMember removes a user from an access group.
func (h *Handler) HandleAdminRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	if err := h.groups.RemoveMember(r.Context(), r.PathValue("id"), r.PathValue("userId")); err != nil {
		writeGroupError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleAdminListUserGroups lists the access groups assigned to a user.
func (h *Handler) HandleAdminListUserGroups(w http.ResponseWriter, r *http.Request) {
	result, err := h.groups.ListUserGroups(r.Context(), r.PathValue("id"))
	if err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleAdminReplaceUserGroups replaces all access group assignments for a user.
func (h *Handler) HandleAdminReplaceUserGroups(w http.ResponseWriter, r *http.Request) {
	var input groups.ReplaceUserGroupsInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	result, err := h.groups.ReplaceUserGroups(r.Context(), r.PathValue("id"), input.GroupIDs)
	if err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleAdminValidateGroupModelPatterns previews which provider models match access group patterns.
func (h *Handler) HandleAdminValidateGroupModelPatterns(w http.ResponseWriter, r *http.Request) {
	var input groups.ValidateModelPatternsInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	patterns, err := compileModelPatterns(input.ModelPatterns)
	if err != nil {
		writeGroupError(w, groups.ErrInvalidRegex)
		return
	}
	if h.providers == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "provider service unavailable"})
		return
	}

	catalog, err := h.providers.ModelCatalog(r.Context(), input.ProviderIDs)
	if err != nil {
		if errors.Is(err, provider.ErrProviderNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider_not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	response := buildModelPatternValidationResponse(catalog, patterns)
	writeJSON(w, http.StatusOK, response)
}

// writeGroupError maps group domain errors to HTTP responses.
func writeGroupError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := err.Error()

	switch {
	case errors.Is(err, groups.ErrGroupNotFound):
		status = http.StatusNotFound
		code = "group_not_found"
	case errors.Is(err, groups.ErrNameConflict):
		status = http.StatusConflict
		code = "name_conflict"
	case errors.Is(err, groups.ErrInvalidName):
		status = http.StatusBadRequest
		code = "invalid_name"
	case errors.Is(err, groups.ErrInvalidDisplayName):
		status = http.StatusBadRequest
		code = "invalid_display_name"
	case errors.Is(err, groups.ErrInvalidRegex):
		status = http.StatusBadRequest
		code = "invalid_regex"
	case errors.Is(err, groups.ErrProviderRequired):
		status = http.StatusBadRequest
		code = "provider_required"
	case errors.Is(err, groups.ErrProviderNotFound):
		status = http.StatusNotFound
		code = "provider_not_found"
	case errors.Is(err, groups.ErrUserNotFound):
		status = http.StatusNotFound
		code = "user_not_found"
	}

	writeJSON(w, status, map[string]string{"error": code})
}

// compileModelPatterns compiles unique non-empty model regex patterns.
func compileModelPatterns(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	seen := map[string]struct{}{}
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		seen[pattern] = struct{}{}
		out = append(out, compiled)
	}
	return out, nil
}

// buildModelPatternValidationResponse summarizes model matches per provider and overall.
func buildModelPatternValidationResponse(catalog []provider.ModelCatalogProvider, patterns []*regexp.Regexp) groups.ModelPatternValidationResponse {
	matchedModels := map[string]struct{}{}
	providerResults := make([]groups.ModelPatternProviderValidationResult, 0, len(catalog))
	unavailableProviderCount := 0

	for _, providerResult := range catalog {
		matches := matchedProviderModels(providerResult.Models, patterns)
		for _, model := range matches {
			matchedModels[model] = struct{}{}
		}
		if providerResult.ModelsError != "" {
			unavailableProviderCount++
		}
		providerResults = append(providerResults, groups.ModelPatternProviderValidationResult{
			ID:                  providerResult.ID,
			Name:                providerResult.Name,
			DisplayName:         providerResult.DisplayName,
			AvailableModelCount: len(providerResult.Models),
			MatchedModelCount:   len(matches),
			MatchedModels:       matches,
			ModelsError:         providerResult.ModelsError,
		})
	}

	models := make([]string, 0, len(matchedModels))
	for model := range matchedModels {
		models = append(models, model)
	}
	sort.Strings(models)
	return groups.ModelPatternValidationResponse{
		MatchedModelCount:        len(models),
		MatchedModels:            models,
		ProviderResults:          providerResults,
		UnavailableProviderCount: unavailableProviderCount,
	}
}

// matchedProviderModels returns sorted unique models matching at least one pattern.
func matchedProviderModels(models []string, patterns []*regexp.Regexp) []string {
	matches := map[string]struct{}{}
	for _, model := range models {
		for _, pattern := range patterns {
			if pattern.MatchString(model) {
				matches[model] = struct{}{}
				break
			}
		}
	}
	out := make([]string, 0, len(matches))
	for model := range matches {
		out = append(out, model)
	}
	sort.Strings(out)
	return out
}
