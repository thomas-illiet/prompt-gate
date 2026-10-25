package runtime

import (
	"context"
	"net/http"
	"sync"
)

// AI Bridge v1.1.2 does not expose an HTTP-client hook for its OpenAI and
// Anthropic SDK interceptors. The SDKs use http.DefaultClient, so Prompt Gate
// installs one context-aware default transport and selects the configured
// provider transport only for requests flowing through Manager. All unrelated
// uses of the process default transport continue through the captured fallback.
// Passthrough routes are owned by routedBridge and use the same transport
// directly; this adapter can be removed once AI Bridge exposes client injection
// consistently for intercepted and passthrough routes.
type providerTransportContextKey struct{}

type contextualProviderTransport struct {
	fallback http.RoundTripper
}

var installContextualProviderTransport sync.Once

func ensureContextualProviderTransport() {
	installContextualProviderTransport.Do(func() {
		fallback := http.DefaultTransport
		if fallback == nil {
			fallback = &http.Transport{Proxy: http.ProxyFromEnvironment}
		}
		http.DefaultTransport = &contextualProviderTransport{fallback: fallback}
	})
}

func (t *contextualProviderTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if transport, ok := r.Context().Value(providerTransportContextKey{}).(http.RoundTripper); ok && transport != nil && transport != t {
		return transport.RoundTrip(r)
	}
	return t.fallback.RoundTrip(r)
}

func withProviderTransport(ctx context.Context, client *http.Client) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil || client.Transport == nil {
		return ctx
	}
	return context.WithValue(ctx, providerTransportContextKey{}, client.Transport)
}

func providerRoundTripper(client *http.Client) http.RoundTripper {
	if client != nil && client.Transport != nil {
		return client.Transport
	}
	return http.DefaultTransport
}
