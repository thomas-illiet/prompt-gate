package httpapi

import (
	"encoding/json"
	"net/http"

	"promptgate/backend/internal/domain/auth"
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
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/transport/httpapi/admin"

	"gorm.io/gorm"
)

type Dependencies struct {
	Config        config.APIConfig
	DB            *gorm.DB
	Users         *users.Service
	Tokens        *tokens.Service
	Firewall      *firewall.Service
	FAQ           *faq.Service
	Groups        *groups.Service
	Providers     *provider.Service
	MCP           *mcp.Service
	Monitoring    *monitoring.Service
	Pricing       *pricing.Service
	Proxy         *proxy.Service
	Subscriptions *subscriptions.Service
	SetupGuides   *setupguide.Service
	QuotaRedis    *subscriptions.RedisStore
	OIDC          *auth.OIDCService
	Sessions      *auth.SessionStore
}

// NewHandler builds and returns the HTTP handler with all routes and middleware configured.
func NewHandler(deps Dependencies) http.Handler {
	srv := &server{
		config:        deps.Config,
		db:            deps.DB,
		oidcService:   deps.OIDC,
		sessionStore:  deps.Sessions,
		userService:   deps.Users,
		tokenService:  deps.Tokens,
		groups:        deps.Groups,
		faq:           deps.FAQ,
		proxyService:  deps.Proxy,
		providers:     deps.Providers,
		monitoring:    deps.Monitoring,
		pricing:       deps.Pricing,
		subscriptions: deps.Subscriptions,
		setupGuides:   deps.SetupGuides,
		quotaRedis:    deps.QuotaRedis,
	}
	adminHandler := admin.NewHandler(admin.Dependencies{
		Users:         deps.Users,
		Tokens:        deps.Tokens,
		Firewall:      deps.Firewall,
		FAQ:           deps.FAQ,
		Groups:        deps.Groups,
		Providers:     deps.Providers,
		MCP:           deps.MCP,
		Monitoring:    deps.Monitoring,
		Pricing:       deps.Pricing,
		Proxy:         deps.Proxy,
		Subscriptions: deps.Subscriptions,
		SetupGuides:   deps.SetupGuides,
	})

	mux := http.NewServeMux()
	registerRoutes(mux, publicRoutes(srv))
	registerRoutes(mux, sessionRoutes(), sessionAccessMiddlewares(deps.Sessions, deps.Config)...)
	registerRoutes(mux, userRoutes(srv), userAccessMiddlewares(deps.Sessions, deps.Config)...)
	registerRoutes(mux, adminRoutes(adminHandler), adminAccessMiddlewares(deps.Sessions, deps.Config)...)

	if deps.Config.StaticAssetsDir != "" {
		mux.Handle("/", newStaticAssetsHandler(deps.Config.StaticAssetsDir))
	}

	return withCommonMiddlewares(mux, deps.Config)
}

// handleCurrentUser returns the authenticated user's profile from the request context.
func handleCurrentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "missing authenticated user in request context",
		})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// writeJSON sets Content-Type to application/json, writes statusCode, and encodes payload.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
