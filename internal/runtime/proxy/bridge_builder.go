package runtime

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	coderbridge "github.com/coder/aibridge"

	localprovider "promptgate/backend/internal/domain/provider"
)

type managedBridge interface {
	http.Handler
	Shutdown(context.Context) error
}

type bridgeBuilder func(context.Context, []coderbridge.Provider) (managedBridge, error)

func (m *Manager) newBridge(ctx context.Context, providers []coderbridge.Provider) (managedBridge, error) {
	ensureContextualProviderTransport()
	router := http.NewServeMux()
	bridgedProviders := make([]coderbridge.Provider, 0, len(providers))
	for _, provider := range providers {
		passthroughRoutes := provider.PassthroughRoutes()
		if len(passthroughRoutes) > 0 {
			passthrough, err := newProviderPassthroughProxy(provider, m.opts.HTTPClient, m.opts.Logger)
			if err != nil {
				return nil, err
			}
			for _, path := range passthroughRoutes {
				route, err := url.JoinPath(provider.RoutePrefix(), path)
				if err != nil {
					return nil, fmt.Errorf("join provider %q passthrough route: %w", provider.Name(), err)
				}
				router.Handle(route, passthrough)
			}
		}
		bridgedProviders = append(bridgedProviders, providerWithoutPassthrough{Provider: provider})
	}

	bridge, err := coderbridge.NewRequestBridge(
		ctx,
		bridgedProviders,
		m.opts.Recorder,
		nil,
		m.opts.BridgeLogger,
		nil,
		m.opts.Tracer,
	)
	if err != nil {
		return nil, err
	}
	router.Handle("/", bridge)
	return &routedBridge{handler: router, inner: bridge}, nil
}

type providerWithoutPassthrough struct {
	coderbridge.Provider
}

func (providerWithoutPassthrough) PassthroughRoutes() []string { return nil }

type routedBridge struct {
	handler http.Handler
	inner   managedBridge
}

func (b *routedBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.handler.ServeHTTP(w, r)
}

func (b *routedBridge) Shutdown(ctx context.Context) error {
	return b.inner.Shutdown(ctx)
}

func newProviderPassthroughProxy(provider coderbridge.Provider, client *http.Client, logger interface {
	Warn(msg string, args ...any)
}) (http.Handler, error) {
	baseURL, err := url.Parse(provider.BaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse provider %q base URL: %w", provider.Name(), err)
	}
	prefix := provider.RoutePrefix()
	proxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			suffix := strings.TrimPrefix(request.In.URL.Path, prefix)
			upstreamPath, joinErr := url.JoinPath(baseURL.Path, suffix)
			if joinErr != nil {
				// Provider URLs and registered routes are validated while building the
				// bridge, so this is defensive. An invalid path is made unroutable and
				// handled by the transport rather than forwarding to the wrong endpoint.
				upstreamPath = "/"
			}
			if !strings.HasPrefix(upstreamPath, "/") {
				upstreamPath = "/" + upstreamPath
			}
			request.Out.URL.Scheme = baseURL.Scheme
			request.Out.URL.Host = baseURL.Host
			request.Out.URL.Path = upstreamPath
			request.Out.URL.RawPath = ""
			request.Out.URL.RawQuery = request.In.URL.RawQuery
			request.Out.URL.ForceQuery = request.In.URL.ForceQuery
			request.Out.Host = baseURL.Host
			request.Out.Header.Del("Authorization")
			request.Out.Header.Del("X-Api-Key")
			// Passthrough model catalogs are inspected and filtered by the groups
			// middleware. Keep their wire representation uncompressed so the
			// bounded response capture cannot be bypassed by compressed content.
			request.Out.Header.Set("Accept-Encoding", "identity")
			provider.InjectAuthHeader(&request.Out.Header)
		},
		Transport: providerRoundTripper(client),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, proxyErr error) {
			logger.Warn("provider passthrough request failed", "provider", provider.Name(), "path", r.URL.Path, "error", proxyErr)
			http.Error(w, "upstream proxy error", http.StatusBadGateway)
		},
	}
	return proxy, nil
}

func (m *Manager) buildProviderRecords(records []localprovider.Provider) ([]coderbridge.Provider, error) {
	runtimeOpts := m.providerRuntimeOptions()
	providers := make([]coderbridge.Provider, 0, len(records))
	for _, record := range records {
		provider, err := m.buildProvider(record, runtimeOpts)
		if err != nil {
			return nil, err
		}
		if provider != nil {
			providers = append(providers, provider)
		}
	}
	return providers, nil
}

func (m *Manager) buildProvider(record localprovider.Provider, opts providerRuntimeOptions) (coderbridge.Provider, error) {
	// AI Bridge historically enables raw request/response dumps from this
	// environment variable when no explicit directory is supplied. Prompt Gate
	// never exposes raw API dumps, so clear the legacy upstream switch before
	// every initial build and hot reload.
	if err := os.Unsetenv("BRIDGE_DUMP_DIR"); err != nil {
		return nil, fmt.Errorf("disable AI Bridge API dumps: %w", err)
	}

	apiKey, err := m.opts.Providers.DecryptAPIKey(record)
	if err != nil {
		return nil, fmt.Errorf("decrypt provider %q api key: %w", record.Name, err)
	}
	switch record.Type {
	case localprovider.ProviderTypeOpenAI:
		return newOpenAIProvider(record.Name, record.BaseURL, apiKey, opts), nil
	case localprovider.ProviderTypeAnthropic:
		return newAnthropicProvider(record.Name, record.BaseURL, apiKey), nil
	case localprovider.ProviderTypeOllama:
		return newOllamaProvider(record.Name, record.BaseURL, apiKey, opts), nil
	default:
		m.opts.Logger.Warn("unsupported provider ignored", "name", record.Name, "type", record.Type)
		return nil, nil
	}
}

func (m *Manager) providerRuntimeOptions() providerRuntimeOptions {
	return normalizeProviderRuntimeOptions(providerRuntimeOptions{
		httpClient:               m.opts.HTTPClient,
		maxBufferedRequestBytes:  m.opts.MaxBufferedRequestBytes,
		maxBufferedResponseBytes: m.opts.MaxBufferedResponseBytes,
	})
}
