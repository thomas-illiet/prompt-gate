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

func (p *openAIProvider) Type() string {
	return p.inner.Type()
}

func (p *openAIProvider) Name() string {
	return p.inner.Name()
}

func (p *openAIProvider) BaseURL() string {
	return p.inner.BaseURL()
}

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

func (p *openAIProvider) RoutePrefix() string {
	return p.inner.RoutePrefix()
}

func (p *openAIProvider) BridgedRoutes() []string {
	routes := append([]string{}, p.inner.BridgedRoutes()...)
	return append(routes, routeEmbeddings)
}

func (p *openAIProvider) PassthroughRoutes() []string {
	return p.inner.PassthroughRoutes()
}

func (p *openAIProvider) AuthHeader() string {
	return p.inner.AuthHeader()
}

func (p *openAIProvider) InjectAuthHeader(headers *http.Header) {
	p.inner.InjectAuthHeader(headers)
}

func (p *openAIProvider) CircuitBreakerConfig() *config.CircuitBreaker {
	return p.inner.CircuitBreakerConfig()
}

func (p *openAIProvider) APIDumpDir() string {
	return p.inner.APIDumpDir()
}
