package app

import (
	"context"
	"net/http"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/config"
	httpmiddleware "promptgate/backend/internal/transport/httpmiddleware"
)

func (p *ProxyRuntime) buildHandler(
	cfg config.ProxyConfig,
	tokenService *tokens.Service,
	userService *users.Service,
	authCache tokens.AuthCache,
	firewallSnapshot *firewall.SnapshotStore,
	accessSnapshot *groups.SnapshotStore,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", proxyHealth)

	clientIPOptions := clientip.Options{
		TrustForwardHeaders: cfg.ProxyTrustForwardHeaders,
		TrustedProxies:      cfg.ProxyTrustedProxies,
	}
	proxyHandler := tokens.MiddlewareWithOptions(tokens.MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        authCache,
		Logger:       p.logger,
	})(
		clientip.MiddlewareWithOptions(clientIPOptions)(
			firewall.MiddlewareWithOptions(firewallSnapshot, clientIPOptions, p.logger)(
				groups.MiddlewareWithOptions(accessSnapshot, p.logger, groups.MiddlewareOptions{
					MaxBufferedRequestBytes: cfg.ProxyMaxBufferedRequestBytes,
				})(
					subscriptions.Middleware(p.subscriptionStore, p.logger)(
						auth.ActorMiddleware(p.manager),
					),
				),
			),
		),
	)
	if len(cfg.CORSAllowedOrigins) > 0 {
		proxyHandler = httpmiddleware.CORS(cfg.CORSAllowedOrigins)(proxyHandler)
	}
	proxyHandler = requestTimeout(cfg.ProxyUpstreamTimeout)(proxyHandler)
	mux.Handle("/", proxyHandler)
	return httpmiddleware.SecurityHeaders()(mux)
}

func proxyHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// requestTimeout bounds a complete proxy request while preserving streaming.
func requestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
