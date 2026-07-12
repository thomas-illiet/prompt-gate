package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"promptgate/backend/internal/domain/faq"
	"promptgate/backend/internal/domain/firewall"
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
	firewall      *firewall.Service
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

// NewHandler returns an admin Handler wired to the given services.
func NewHandler(u *users.Service, t *tokens.Service, f *firewall.Service, g *groups.Service, p *provider.Service, m *mcp.Service, optionalServices ...any) *Handler {
	handler := &Handler{users: u, tokens: t, firewall: f, groups: g, providers: p, mcp: m}
	for _, service := range optionalServices {
		switch typed := service.(type) {
		case *proxy.Service:
			handler.proxy = typed
		case *monitoring.Service:
			handler.monitoring = typed
		case *subscriptions.Service:
			handler.subscriptions = typed
		case *pricing.Service:
			handler.pricing = typed
		case *setupguide.Service:
			handler.setupGuides = typed
		case *faq.Service:
			handler.faq = typed
		}
	}
	return handler
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
