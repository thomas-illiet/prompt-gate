package httpapi

import (
	"log/slog"
	"net/http"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/transport/httpmiddleware"
)

// routeDefinition keeps the HTTP contract separate from handler construction.
type routeDefinition struct {
	pattern string
	handler http.HandlerFunc
}

func registerRoutes(mux *http.ServeMux, routes []routeDefinition, middlewares ...middleware.Middleware) {
	for _, route := range routes {
		mux.Handle(route.pattern, middleware.Chain(route.handler, middlewares...))
	}
}

func sessionAccessMiddlewares(store *auth.SessionStore, cfg config.APIConfig) []middleware.Middleware {
	return []middleware.Middleware{
		middleware.RequireSession(store, cfg.SessionCookieName),
		middleware.RequireAppAccess(),
	}
}

func userAccessMiddlewares(store *auth.SessionStore, cfg config.APIConfig) []middleware.Middleware {
	return append(
		sessionAccessMiddlewares(store, cfg),
		middleware.RequireRoles(auth.RoleUser, auth.RoleManager, auth.RoleAdmin),
	)
}

func adminAccessMiddlewares(store *auth.SessionStore, cfg config.APIConfig) []middleware.Middleware {
	return []middleware.Middleware{
		middleware.RequireAdminAccess(store, cfg.SessionCookieName, cfg.AdminAPIKey),
	}
}

func withCommonMiddlewares(handler http.Handler, cfg config.APIConfig) http.Handler {
	return middleware.Chain(
		handler,
		middleware.RequestLogger(slog.Default()),
		middleware.SecurityHeaders(),
		middleware.CORS(cfg.CORSAllowedOrigins),
	)
}
