package app

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strings"
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
	return requestprivacy.Middleware(httpmiddleware.SecurityHeaders()(proxyBasePathMiddleware(cfg.ProxyBasePath, mux)))
}

// proxyBasePathMiddleware mounts the proxy below an optional public path while
// keeping the direct health endpoint available to Kubernetes probes. The path
// is normalized before any auth, policy, quota, or provider middleware sees it.
func proxyBasePathMiddleware(basePath string, next http.Handler) http.Handler {
	if basePath == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isCanonicalProxyPath(r.URL) {
			http.NotFound(w, r)
			return
		}

		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		var strippedPath string
		switch {
		case r.URL.Path == basePath:
			strippedPath = "/"
		case strings.HasPrefix(r.URL.Path, basePath+"/"):
			strippedPath = strings.TrimPrefix(r.URL.Path, basePath)
		default:
			http.NotFound(w, r)
			return
		}

		normalized := r.Clone(r.Context())
		normalized.URL.Path = strippedPath
		normalized.URL.RawPath = ""
		normalized.RequestURI = normalized.URL.RequestURI()
		next.ServeHTTP(w, normalized)
	})
}

// isCanonicalProxyPath rejects paths that net/http's ServeMux would clean and
// redirect after the public base path has been removed. It also rejects
// escaped dots and separators, whose routing semantics can differ between
// intermediaries even when RawPath decodes to Path exactly.
func isCanonicalProxyPath(requestURL *url.URL) bool {
	requestPath := requestURL.Path
	if requestPath == "" || requestPath[0] != '/' || strings.ContainsRune(requestPath, '\\') {
		return false
	}

	cleanedPath := path.Clean(requestPath)
	if strings.HasSuffix(requestPath, "/") && cleanedPath != "/" {
		cleanedPath += "/"
	}
	if cleanedPath != requestPath {
		return false
	}

	if requestURL.RawPath == "" {
		return true
	}
	decodedRawPath, err := url.PathUnescape(requestURL.RawPath)
	if err != nil || decodedRawPath != requestPath || requestURL.EscapedPath() != requestURL.RawPath {
		return false
	}
	return !hasAmbiguousPathEscape(requestURL.RawPath)
}

func hasAmbiguousPathEscape(rawPath string) bool {
	for index := 0; index+2 < len(rawPath); index++ {
		if rawPath[index] != '%' {
			continue
		}
		high, highOK := hexNibble(rawPath[index+1])
		low, lowOK := hexNibble(rawPath[index+2])
		if !highOK || !lowOK {
			return true
		}
		switch high<<4 | low {
		case '.', '/', '\\':
			return true
		}
		index += 2
	}
	return false
}

func hexNibble(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
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
