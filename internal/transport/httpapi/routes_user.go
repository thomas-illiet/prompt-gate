package httpapi

func sessionRoutes() []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/me", handler: handleCurrentUser},
	}
}

func userRoutes(srv *server) []routeDefinition {
	routes := make([]routeDefinition, 0, 17)
	routes = append(routes, currentUserRoutes(srv)...)
	routes = append(routes, userMonitoringRoutes(srv)...)
	routes = append(routes, userFAQRoutes(srv)...)
	routes = append(routes, userTokenRoutes(srv)...)
	return routes
}

func currentUserRoutes(srv *server) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/me/usage", handler: srv.handleCurrentUserUsage},
		{pattern: "GET /api/v1/me/prompts", handler: srv.handleCurrentUserPrompts},
		{pattern: "GET /api/v1/me/quota", handler: srv.handleCurrentUserQuota},
		{pattern: "GET /api/v1/me/help/setup", handler: srv.handleHelpSetup},
		{pattern: "GET /api/v1/me/dashboard/tokens", handler: srv.handleCurrentUserDashboardTokens},
		{pattern: "GET /api/v1/me/dashboard/messages", handler: srv.handleCurrentUserDashboardMessages},
		{pattern: "GET /api/v1/me/dashboard/duration", handler: srv.handleCurrentUserDashboardDuration},
		{pattern: "GET /api/v1/me/dashboard/activity", handler: srv.handleCurrentUserDashboardActivity},
		{pattern: "GET /api/v1/me/dashboard/top-models", handler: srv.handleCurrentUserDashboardTopModels},
		{pattern: "GET /api/v1/me/dashboard/top-provider-names", handler: srv.handleCurrentUserDashboardTopProviderNames},
		{pattern: "GET /api/v1/me/dashboard/top-provider-types", handler: srv.handleCurrentUserDashboardTopProviderTypes},
		{pattern: "GET /api/v1/me/groups", handler: srv.handleCurrentUserGroups},
	}
}

func userMonitoringRoutes(srv *server) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/monitoring/status", handler: srv.handleMonitoringStatus},
	}
}

func userFAQRoutes(srv *server) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/faq", handler: srv.handleFAQ},
	}
}

func userTokenRoutes(srv *server) []routeDefinition {
	return []routeDefinition{
		{pattern: "POST /api/v1/tokens", handler: srv.handleCreateToken},
		{pattern: "GET /api/v1/tokens", handler: srv.handleListTokens},
		{pattern: "DELETE /api/v1/tokens/{id}", handler: srv.handleRevokeToken},
	}
}
