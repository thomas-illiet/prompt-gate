package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/modellab"
	"promptgate/backend/internal/platform/requestprivacy"
)

func (s server) handleModelLabCatalog(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(requestprivacy.MarkMetadataOnly(r.Context()))
	setModelLabNoStore(w)
	user, access, ok := s.modelLabRequestAccess(w, r)
	if !ok {
		return
	}

	catalog, err := s.modelLab.Catalog(r.Context(), modelLabAccessFilter(access))
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "model_lab_catalog_unavailable"})
		return
	}
	_ = user
	writeJSON(w, http.StatusOK, catalog)
}

func (s server) handleIssueModelLabTickets(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(requestprivacy.MarkMetadataOnly(r.Context()))
	setModelLabNoStore(w)
	user, access, ok := s.modelLabRequestAccess(w, r)
	if !ok {
		return
	}

	var input modellab.IssueTicketsInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_model_lab_targets"})
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_model_lab_targets"})
		return
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		origin = s.config.FrontendBaseURL
	}
	if !modelLabOriginAllowed(origin, s.config.CORSAllowedOrigins) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "model_lab_origin_not_allowed"})
		return
	}

	result, err := s.modelLab.IssueTickets(
		r.Context(),
		user.ID,
		string(user.Role),
		origin,
		input,
		modelLabAccessFilter(access),
	)
	if err != nil {
		writeModelLabIssueError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s server) modelLabRequestAccess(w http.ResponseWriter, r *http.Request) (auth.UserProfile, groups.UserAccess, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing_user_in_context"})
		return auth.UserProfile{}, groups.UserAccess{}, false
	}
	if s.modelLab == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "model_lab_unavailable"})
		return auth.UserProfile{}, groups.UserAccess{}, false
	}
	if s.groups == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "model_lab_catalog_unavailable"})
		return auth.UserProfile{}, groups.UserAccess{}, false
	}
	access, err := s.groups.UserAccess(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "model_lab_catalog_unavailable"})
		return auth.UserProfile{}, groups.UserAccess{}, false
	}
	return user, access, true
}

func modelLabAccessFilter(access groups.UserAccess) modellab.AccessFilter {
	providers := make(map[string]struct{}, len(access.Providers))
	for _, name := range access.Providers {
		providers[name] = struct{}{}
	}
	return modellab.AccessFilter{
		ProviderAllowed: func(providerName string) bool {
			_, ok := providers[providerName]
			return ok
		},
		ModelAllowed: access.Allows,
	}
}

func writeModelLabIssueError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "model_lab_ticket_unavailable"
	switch {
	case errors.Is(err, modellab.ErrInvalidTargets):
		status, code = http.StatusBadRequest, "invalid_model_lab_targets"
	case errors.Is(err, modellab.ErrTargetNotAllowed):
		status, code = http.StatusForbidden, "model_lab_target_not_allowed"
	case errors.Is(err, modellab.ErrCatalogUnavailable):
		status, code = http.StatusServiceUnavailable, "model_lab_catalog_unavailable"
	case errors.Is(err, modellab.ErrTicketUnavailable):
		status, code = http.StatusServiceUnavailable, "model_lab_ticket_unavailable"
	case errors.Is(err, modellab.ErrTicketIssueLimit):
		status, code = http.StatusTooManyRequests, "model_lab_ticket_issue_limit"
	}
	writeJSON(w, status, map[string]string{"error": code})
}

func setModelLabNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Pragma", "no-cache")
}

func modelLabOriginAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, candidate := range allowed {
		if origin == strings.TrimRight(strings.TrimSpace(candidate), "/") {
			return true
		}
	}
	return false
}
