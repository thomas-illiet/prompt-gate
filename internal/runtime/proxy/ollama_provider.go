package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/coder/aibridge/config"
	"github.com/coder/aibridge/intercept"
	"github.com/coder/aibridge/intercept/chatcompletions"
	aibprovider "github.com/coder/aibridge/provider"
	"github.com/coder/aibridge/tracing"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ollamaProvider adapts Ollama's OpenAI-compatible API to promptgate.
type ollamaProvider struct {
	baseURL string
	key     string
	name    string
}

// newOllamaProvider creates an Ollama provider adapter with a normalized base URL.
func newOllamaProvider(name, baseURL, key string) *ollamaProvider {
	return &ollamaProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		name:    name,
	}
}

// Type identifies this provider as Ollama.
func (*ollamaProvider) Type() string {
	return "ollama"
}

// Name returns the configured provider name.
func (p *ollamaProvider) Name() string {
	return p.name
}

// BaseURL returns the upstream Ollama base URL.
func (p *ollamaProvider) BaseURL() string {
	return p.baseURL
}

// APIDumpDir disables API dump output for the Ollama adapter.
func (*ollamaProvider) APIDumpDir() string {
	return ""
}

// CircuitBreakerConfig leaves circuit breaker settings at the bridge default.
func (*ollamaProvider) CircuitBreakerConfig() *config.CircuitBreaker {
	return nil
}

// AuthHeader returns the header used for upstream Ollama authorization.
func (*ollamaProvider) AuthHeader() string {
	return "Authorization"
}

// RoutePrefix returns the public route prefix for this Ollama provider.
func (p *ollamaProvider) RoutePrefix() string {
	return fmt.Sprintf("/%s/v1", p.name)
}

// BridgedRoutes returns the Ollama routes intercepted by the bridge.
func (*ollamaProvider) BridgedRoutes() []string {
	return []string{"/chat/completions", routeEmbeddings}
}

// PassthroughRoutes returns Ollama routes proxied without interception.
func (*ollamaProvider) PassthroughRoutes() []string {
	return []string{"/models", "/models/"}
}

// InjectAuthHeader sets the authorization header expected by Ollama.
func (p *ollamaProvider) InjectAuthHeader(headers *http.Header) {
	if headers == nil {
		return
	}

	if p.key == "" {
		headers.Set("Authorization", "Bearer ollama")
		return
	}

	headers.Set("Authorization", "Bearer "+p.key)
}

// CreateInterceptor creates the blocking or streaming chat completion interceptor.
func (p *ollamaProvider) CreateInterceptor(
	_ http.ResponseWriter,
	r *http.Request,
	tracer trace.Tracer,
) (_ intercept.Interceptor, outErr error) {
	id := uuid.New()

	_, span := tracer.Start(r.Context(), "Intercept.CreateInterceptor")
	defer tracing.EndSpanErr(span, &outErr)

	path := strings.TrimPrefix(r.URL.Path, p.RoutePrefix())
	if path != "/chat/completions" && path != routeEmbeddings {
		span.SetStatus(codes.Error, "unknown route: "+r.URL.Path)
		return nil, aibprovider.ErrUnknownRoute
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	if path == routeEmbeddings {
		interceptor := newEmbeddingInterceptor(p, raw, tracer, p.key)
		span.SetAttributes(interceptor.TraceAttributes(r)...)
		return interceptor, nil
	}

	cfg := config.OpenAI{
		Name:    p.name,
		BaseURL: p.baseURL,
		Key:     p.key,
	}
	cred := intercept.NewCredentialInfo(
		intercept.CredentialKindCentralized,
		cfg.Key,
	)

	var req chatcompletions.ChatCompletionNewParamsWrapper
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("unmarshal request body: %w", err)
	}

	var interceptor intercept.Interceptor
	if req.Stream {
		interceptor = chatcompletions.NewStreamingInterceptor(
			id,
			&req,
			p.name,
			cfg,
			r.Header,
			p.AuthHeader(),
			tracer,
			cred,
		)
	} else {
		interceptor = chatcompletions.NewBlockingInterceptor(
			id,
			&req,
			p.name,
			cfg,
			r.Header,
			p.AuthHeader(),
			tracer,
			cred,
		)
	}

	span.SetAttributes(interceptor.TraceAttributes(r)...)
	return interceptor, nil
}
