package runtime

import (
	"io"
	"net/http"
	"strings"

	coderbridge "github.com/coder/aibridge"
	"github.com/coder/aibridge/config"
	"github.com/coder/aibridge/intercept"
	"go.opentelemetry.io/otel/trace"
)

const routeEmbeddings = "/embeddings"

// openAIProvider extends aibridge's OpenAI provider with embeddings support.
type openAIProvider struct {
	inner coderbridge.Provider
	key   string
}

// newOpenAIProvider wraps the aibridge OpenAI provider with Prompt Gate extensions.
func newOpenAIProvider(name, baseURL, key string) *openAIProvider {
	return &openAIProvider{
		inner: coderbridge.NewOpenAIProvider(coderbridge.OpenAIConfig{
			Name:    name,
			BaseURL: baseURL,
			Key:     key,
		}),
		key: key,
	}
}

// Type returns the wrapped provider type.
func (p *openAIProvider) Type() string {
	return p.inner.Type()
}

// Name returns the configured provider name.
func (p *openAIProvider) Name() string {
	return p.inner.Name()
}

// BaseURL returns the upstream provider base URL.
func (p *openAIProvider) BaseURL() string {
	return p.inner.BaseURL()
}

// CreateInterceptor creates a standard interceptor or an embeddings interceptor for embedding routes.
func (p *openAIProvider) CreateInterceptor(w http.ResponseWriter, r *http.Request, tracer trace.Tracer) (intercept.Interceptor, error) {
	path := strings.TrimPrefix(r.URL.Path, p.RoutePrefix())
	if path != routeEmbeddings {
		return p.inner.CreateInterceptor(w, r, tracer)
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return newEmbeddingInterceptor(p, raw, tracer, p.key), nil
}

// RoutePrefix returns the provider route prefix handled by the proxy.
func (p *openAIProvider) RoutePrefix() string {
	return p.inner.RoutePrefix()
}

// BridgedRoutes returns routes intercepted by aibridge plus embeddings.
func (p *openAIProvider) BridgedRoutes() []string {
	routes := append([]string{}, p.inner.BridgedRoutes()...)
	return append(routes, routeEmbeddings)
}

// PassthroughRoutes returns routes forwarded without bridge interception.
func (p *openAIProvider) PassthroughRoutes() []string {
	return p.inner.PassthroughRoutes()
}

// AuthHeader returns the upstream authentication header name.
func (p *openAIProvider) AuthHeader() string {
	return p.inner.AuthHeader()
}

// InjectAuthHeader adds upstream authentication to request headers.
func (p *openAIProvider) InjectAuthHeader(headers *http.Header) {
	p.inner.InjectAuthHeader(headers)
}

// CircuitBreakerConfig returns the wrapped provider circuit breaker configuration.
func (p *openAIProvider) CircuitBreakerConfig() *config.CircuitBreaker {
	return p.inner.CircuitBreakerConfig()
}

// APIDumpDir returns the wrapped provider API dump directory.
func (p *openAIProvider) APIDumpDir() string {
	return p.inner.APIDumpDir()
}
