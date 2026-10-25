package app

import (
	"context"
	"net/http"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/modellab"
	"promptgate/backend/internal/domain/policy"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/requestprivacy"
	proxyruntime "promptgate/backend/internal/runtime/proxy"
	httpmiddleware "promptgate/backend/internal/transport/httpmiddleware"
)

func (p *ProxyRuntime) buildHandler(
	cfg config.ProxyConfig,
	tokenService *tokens.Service,
	userService *users.Service,
	authCache tokens.AuthCache,
	policyEvaluator *policy.Evaluator,
	opaClient *policy.Client,
	accessSnapshot *groups.SnapshotStore,
	denialRecorder policy.DenialRecorder,
	modelLabService *modellab.Service,
	debugRequestWriter *httpmiddleware.JSONLineWriter,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", proxyHealth(cfg.OPAEnabled, opaClient))

	clientIPOptions := clientip.Options{
		TrustForwardHeaders: cfg.ProxyTrustForwardHeaders,
		TrustedProxies:      cfg.ProxyTrustedProxies,
		Debug:               cfg.ProxyClientIPDebug,
		Logger:              p.logger,
	}
	protectedHandler := groups.MiddlewareWithOptions(accessSnapshot, p.logger, groups.MiddlewareOptions{
		MaxBufferedRequestBytes:  cfg.ProxyMaxBufferedRequestBytes,
		MaxBufferedResponseBytes: cfg.ProxyMaxBufferedResponseBytes,
	})(
		subscriptions.MiddlewareWithOptions(p.subscriptionStore, p.logger, subscriptions.MiddlewareOptions{
			MaxBufferedRequestBytes:  cfg.ProxyMaxBufferedRequestBytes,
			MaxBufferedResponseBytes: cfg.ProxyMaxBufferedResponseBytes,
			ReservationTTL:           cfg.ProxyUpstreamTimeout + time.Minute,
		})(
			auth.ActorMiddleware(proxyruntime.SessionContextMiddleware(p.manager)),
		),
	)
	protectedHandler = stripModelLabClientHeaders(protectedHandler)
	if cfg.OPAEnabled {
		protectedHandler = policy.Middleware(policyEvaluator, accessSnapshot, denialRecorder, p.logger)(protectedHandler)
	}
	protectedHandler = proxyruntime.MethodGuardMiddlewareWithOptions(proxyruntime.MethodGuardOptions{
		MaxBufferedRequestBytes: cfg.ProxyMaxBufferedRequestBytes,
	})(protectedHandler)
	proxyHandler := tokens.MiddlewareWithOptions(tokens.MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        authCache,
		Logger:       p.logger,
	})(
		clientip.MiddlewareWithOptions(clientIPOptions)(protectedHandler),
	)
	proxyHandler = modelLabTicketMiddleware(cfg.ModelLab, modelLabService, userService)(proxyHandler)
	if len(cfg.CORSAllowedOrigins) > 0 {
		proxyHandler = httpmiddleware.CORS(cfg.CORSAllowedOrigins)(proxyHandler)
	}
	if cfg.OTel.CaptureOutput || cfg.OTel.CaptureThinking {
		proxyHandler = proxyruntime.OutputCaptureMiddleware(
			cfg.ProxyMaxBufferedResponseBytes,
			cfg.OTel.CaptureOutput,
			cfg.OTel.CaptureThinking,
		)(proxyHandler)
	}
	proxyHandler = requestTimeout(cfg.ProxyUpstreamTimeout)(proxyHandler)
	proxyHandler = proxyruntime.ResponseTimingMiddleware(proxyHandler)
	if debugRequestWriter != nil {
		proxyHandler = debugRequestWriter.DebugRequests(cfg.ProxyMaxBufferedRequestBytes)(proxyHandler)
	}
	mux.Handle("/", proxyHandler)
	return requestprivacy.Middleware(httpmiddleware.SecurityHeaders()(mux))
}

func proxyHealth(opaEnabled bool, opaClient *policy.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if opaEnabled {
			if opaClient == nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"degraded","dependency":"opa"}`))
				return
			}
			if err := opaClient.Health(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"degraded","dependency":"opa"}`))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
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
