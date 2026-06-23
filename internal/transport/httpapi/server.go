package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/mcp"
	"promptgate/backend/internal/domain/monitoring"
	"promptgate/backend/internal/domain/pricing"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/transport/httpapi/admin"
	"promptgate/backend/internal/transport/httpmiddleware"

	"gorm.io/gorm"
)

type Dependencies struct {
	Config        config.Config
	DB            *gorm.DB
	Users         *users.Service
	Tokens        *tokens.Service
	Firewall      *firewall.Service
	Groups        *groups.Service
	Providers     *provider.Service
	MCP           *mcp.Service
	Monitoring    *monitoring.Service
	Pricing       *pricing.Service
	Proxy         *proxy.Service
	Subscriptions *subscriptions.Service
	QuotaRedis    *subscriptions.RedisStore
	OIDC          *auth.OIDCService
	Sessions      *auth.SessionStore
}

// NewHandler builds and returns the HTTP handler with all routes and middleware configured.
func NewHandler(a Dependencies) http.Handler {
	srv := server{
		config:        a.Config,
		db:            a.DB,
		oidcService:   a.OIDC,
		sessionStore:  a.Sessions,
		userService:   a.Users,
		tokenService:  a.Tokens,
		groups:        a.Groups,
		proxyService:  a.Proxy,
		providers:     a.Providers,
		monitoring:    a.Monitoring,
		pricing:       a.Pricing,
		subscriptions: a.Subscriptions,
		quotaRedis:    a.QuotaRedis,
	}
	adminH := admin.NewHandler(a.Users, a.Tokens, a.Firewall, a.Groups, a.Providers, a.MCP, a.Proxy, a.Monitoring, a.Subscriptions, a.Pricing)

	cfg := a.Config
	sessionStore := a.Sessions

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.handleHealth)
	mux.HandleFunc("GET /auth/login", srv.handleLogin)
	mux.HandleFunc("GET /auth/callback", srv.handleCallback)
	mux.HandleFunc("GET /auth/logout", srv.handleLogout)
	mux.HandleFunc("GET /auth/session", srv.handleSession)
	mux.Handle(
		"GET /api/v1/me",
		middleware.Chain(
			http.HandlerFunc(handleCurrentUser),
			middleware.RequireSession(sessionStore, cfg.SessionCookieName),
			middleware.RequireAppAccess(),
		),
	)
	mux.Handle(
		"GET /api/v1/me/usage",
		middleware.Chain(
			http.HandlerFunc(srv.handleCurrentUserUsage),
			middleware.RequireSession(sessionStore, cfg.SessionCookieName),
			middleware.RequireAppAccess(),
			middleware.RequireRoles(auth.RoleUser, auth.RoleManager, auth.RoleAdmin),
		),
	)
	mux.Handle(
		"GET /api/v1/me/prompts",
		middleware.Chain(
			http.HandlerFunc(srv.handleCurrentUserPrompts),
			middleware.RequireSession(sessionStore, cfg.SessionCookieName),
			middleware.RequireAppAccess(),
			middleware.RequireRoles(auth.RoleUser, auth.RoleManager, auth.RoleAdmin),
		),
	)
	mux.Handle(
		"GET /api/v1/me/quota",
		middleware.Chain(
			http.HandlerFunc(srv.handleCurrentUserQuota),
			middleware.RequireSession(sessionStore, cfg.SessionCookieName),
			middleware.RequireAppAccess(),
			middleware.RequireRoles(auth.RoleUser, auth.RoleManager, auth.RoleAdmin),
		),
	)
	mux.Handle(
		"GET /api/v1/me/help/setup",
		middleware.Chain(
			http.HandlerFunc(srv.handleHelpSetup),
			middleware.RequireSession(sessionStore, cfg.SessionCookieName),
			middleware.RequireAppAccess(),
			middleware.RequireRoles(auth.RoleUser, auth.RoleManager, auth.RoleAdmin),
		),
	)
	mux.Handle(
		"GET /api/v1/monitoring/status",
		middleware.Chain(
			http.HandlerFunc(srv.handleMonitoringStatus),
			middleware.RequireSession(sessionStore, cfg.SessionCookieName),
			middleware.RequireAppAccess(),
			middleware.RequireRoles(auth.RoleUser, auth.RoleManager, auth.RoleAdmin),
		),
	)
	userMiddlewares := []middleware.Middleware{
		middleware.RequireSession(sessionStore, cfg.SessionCookieName),
		middleware.RequireAppAccess(),
		middleware.RequireRoles(auth.RoleUser, auth.RoleManager, auth.RoleAdmin),
	}
	mux.Handle(
		"GET /api/v1/me/dashboard/tokens",
		middleware.Chain(http.HandlerFunc(srv.handleCurrentUserDashboardTokens), userMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/me/dashboard/messages",
		middleware.Chain(http.HandlerFunc(srv.handleCurrentUserDashboardMessages), userMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/me/dashboard/duration",
		middleware.Chain(http.HandlerFunc(srv.handleCurrentUserDashboardDuration), userMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/me/dashboard/activity",
		middleware.Chain(http.HandlerFunc(srv.handleCurrentUserDashboardActivity), userMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/me/dashboard/top-models",
		middleware.Chain(http.HandlerFunc(srv.handleCurrentUserDashboardTopModels), userMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/me/dashboard/top-provider-names",
		middleware.Chain(http.HandlerFunc(srv.handleCurrentUserDashboardTopProviderNames), userMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/me/dashboard/top-provider-types",
		middleware.Chain(http.HandlerFunc(srv.handleCurrentUserDashboardTopProviderTypes), userMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/me/groups",
		middleware.Chain(http.HandlerFunc(srv.handleCurrentUserGroups), userMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/tokens",
		middleware.Chain(http.HandlerFunc(srv.handleCreateToken), userMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/tokens",
		middleware.Chain(http.HandlerFunc(srv.handleListTokens), userMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/tokens/{id}",
		middleware.Chain(http.HandlerFunc(srv.handleRevokeToken), userMiddlewares...),
	)

	adminMiddlewares := []middleware.Middleware{
		middleware.RequireSession(sessionStore, cfg.SessionCookieName),
		middleware.RequireAppAccess(),
		middleware.RequireRoles(auth.RoleAdmin),
	}
	mux.Handle(
		"GET /api/v1/admin/users",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListUsers), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/users/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetUser), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/users/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateUser), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/users/{id}/note",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateUserNote), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/users/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteUser), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/users/{id}/tokens",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListUserTokens), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/users/{id}/tokens/{tokenId}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminRevokeToken), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/users/{id}/groups",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListUserGroups), adminMiddlewares...),
	)
	mux.Handle(
		"PUT /api/v1/admin/users/{id}/groups",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminReplaceUserGroups), adminMiddlewares...),
	)
	mux.Handle(
		"PUT /api/v1/admin/users/{id}/subscription-plan",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminAssignUserSubscriptionPlan), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/prompts",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListPrompts), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/dashboard/tokens",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDashboardTokens), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/dashboard/messages",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDashboardMessages), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/dashboard/duration",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDashboardDuration), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/dashboard/activity",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDashboardActivity), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/dashboard/top-models",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDashboardTopModels), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/dashboard/top-provider-names",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDashboardTopProviderNames), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/dashboard/top-provider-types",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDashboardTopProviderTypes), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/dashboard/adoption",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDashboardAdoption), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/dashboard/top-identities",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDashboardTopIdentities), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/service-accounts",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListServiceAccounts), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/service-accounts",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateServiceAccount), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/service-accounts/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetServiceAccount), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/service-accounts/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateServiceAccount), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/service-accounts/{id}/note",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateServiceAccountNote), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/service-accounts/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteServiceAccount), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/service-accounts/{id}/tokens",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListServiceAccountTokens), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/service-accounts/{id}/tokens",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateServiceAccountToken), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/service-accounts/{id}/tokens/{tokenId}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminRevokeServiceAccountToken), adminMiddlewares...),
	)
	mux.Handle(
		"PUT /api/v1/admin/service-accounts/{id}/subscription-plan",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminAssignServiceAccountSubscriptionPlan), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/service-accounts/{id}/firewall/rules",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListServiceAccountFirewallRules), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/service-accounts/{id}/firewall/rules",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateServiceAccountFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetServiceAccountFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateServiceAccountFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}/priority",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminMoveServiceAccountFirewallRulePriority), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/service-accounts/{id}/firewall/simulate",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminSimulateServiceAccountFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteServiceAccountFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/firewall/rules",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListFirewallRules), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/firewall/rules",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/firewall/rules/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/firewall/rules/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/firewall/rules/{id}/priority",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminMoveFirewallRulePriority), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/firewall/simulate",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminSimulateFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/firewall/rules/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteFirewallRule), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/subscriptions",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListSubscriptionPlans), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/subscriptions",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateSubscriptionPlan), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/subscriptions/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetSubscriptionPlan), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/subscriptions/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateSubscriptionPlan), adminMiddlewares...),
	)
	mux.Handle(
		"PUT /api/v1/admin/subscriptions/{id}/default",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminSetDefaultSubscriptionPlan), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/subscriptions/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteSubscriptionPlan), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/groups",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListGroups), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/groups",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateGroup), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/groups/model-patterns/validate",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminValidateGroupModelPatterns), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/groups/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetGroup), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/groups/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateGroup), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/groups/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteGroup), adminMiddlewares...),
	)
	mux.Handle(
		"PUT /api/v1/admin/groups/{id}/members/{userId}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminAddGroupMember), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/groups/{id}/members/{userId}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminRemoveGroupMember), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/providers",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListProviders), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/providers/model-catalog",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminProviderModelCatalog), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/pricing",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetPricing), adminMiddlewares...),
	)
	mux.Handle(
		"PUT /api/v1/admin/pricing",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdatePricing), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/pricing/fallback",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdatePricingFallback), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/pricing/models",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateModelPrice), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/pricing/models/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetModelPrice), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/pricing/models/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateModelPrice), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/pricing/models/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteModelPrice), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/pricing/check",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminPricingConfigurationCheck), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/providers",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateProvider), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/providers/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetProvider), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/providers/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateProvider), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/providers/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteProvider), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/mcp/servers",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListMCPServers), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/mcp/servers",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateMCPServer), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/mcp/servers/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetMCPServer), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/mcp/servers/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateMCPServer), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/mcp/servers/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteMCPServer), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/monitoring/services",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminListMonitoringServices), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/monitoring/services",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCreateMonitoringService), adminMiddlewares...),
	)
	mux.Handle(
		"GET /api/v1/admin/monitoring/services/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminGetMonitoringService), adminMiddlewares...),
	)
	mux.Handle(
		"PATCH /api/v1/admin/monitoring/services/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminUpdateMonitoringService), adminMiddlewares...),
	)
	mux.Handle(
		"DELETE /api/v1/admin/monitoring/services/{id}",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminDeleteMonitoringService), adminMiddlewares...),
	)
	mux.Handle(
		"POST /api/v1/admin/monitoring/services/{id}/check",
		middleware.Chain(http.HandlerFunc(adminH.HandleAdminCheckMonitoringService), adminMiddlewares...),
	)
	if cfg.StaticAssetsDir != "" {
		mux.Handle("/", newStaticAssetsHandler(cfg.StaticAssetsDir))
	}

	return middleware.Chain(
		mux,
		middleware.RequestLogger(slog.Default()),
		middleware.SecurityHeaders(),
		middleware.CORS(cfg.CORSAllowedOrigins),
	)
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
