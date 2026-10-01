package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"promptgate/backend/internal/domain/faq"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/mcp"
	"promptgate/backend/internal/domain/monitoring"
	"promptgate/backend/internal/domain/pricing"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/setupguide"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
)

// Handler handles admin-only HTTP routes for user and token management.
type Handler struct {
	users         *users.Service
	tokens        *tokens.Service
	faq           *faq.Service
	groups        *groups.Service
	providers     *provider.Service
	mcp           *mcp.Service
	monitoring    *monitoring.Service
	pricing       *pricing.Service
	proxy         *proxy.Service
	subscriptions *subscriptions.Service
	setupGuides   *setupguide.Service
}

func decodeRequestBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request_body"})
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request_body"})
		return false
	}
	return true
}

// Dependencies lists the services consumed by administration handlers.
type Dependencies struct {
	Users         *users.Service
	Tokens        *tokens.Service
	FAQ           *faq.Service
	Groups        *groups.Service
	Providers     *provider.Service
	MCP           *mcp.Service
	Monitoring    *monitoring.Service
	Pricing       *pricing.Service
	Proxy         *proxy.Service
	Subscriptions *subscriptions.Service
	SetupGuides   *setupguide.Service
}

// NewHandler returns an admin Handler wired to explicitly typed dependencies.
func NewHandler(deps Dependencies) *Handler {
	return &Handler{
		users:         deps.Users,
		tokens:        deps.Tokens,
		faq:           deps.FAQ,
		groups:        deps.Groups,
		providers:     deps.Providers,
		mcp:           deps.MCP,
		monitoring:    deps.Monitoring,
		pricing:       deps.Pricing,
		proxy:         deps.Proxy,
		subscriptions: deps.Subscriptions,
		setupGuides:   deps.SetupGuides,
	}
}

// writeJSON sets Content-Type to application/json, writes statusCode, and encodes payload.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

// parsePositiveInt parses a positive integer from raw, returning fallback on empty or non-positive input.
func parsePositiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

type listQuery struct {
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}

// parseListQuery normalizes shared admin pagination and sorting query parameters.
func parseListQuery(r *http.Request, defaultSortBy, defaultSortDir string) listQuery {
	query := r.URL.Query()
	page := parsePositiveInt(query.Get("page"), 1)
	pageSize := parsePositiveInt(query.Get("pageSize"), 10)
	if pageSize > 100 {
		pageSize = 100
	}

	sortBy := query.Get("sortBy")
	if sortBy == "" {
		sortBy = defaultSortBy
	}
	sortDir := query.Get("sortDir")
	if sortDir == "" {
		sortDir = defaultSortDir
	}

	return listQuery{
		Page:     page,
		PageSize: pageSize,
		SortBy:   sortBy,
		SortDir:  sortDir,
	}
}
