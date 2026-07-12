package httpapi

func publicRoutes(srv *server) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /health", handler: srv.handleHealth},
		{pattern: "GET /auth/login", handler: srv.handleLogin},
		{pattern: "GET /auth/callback", handler: srv.handleCallback},
		{pattern: "GET /auth/logout", handler: srv.handleLogout},
		{pattern: "GET /auth/session", handler: srv.handleSession},
	}
}
