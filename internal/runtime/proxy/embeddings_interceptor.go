package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"

	cdrslog "cdr.dev/slog/v3"
	coderbridge "github.com/coder/aibridge"
	aibcontext "github.com/coder/aibridge/context"
	"github.com/coder/aibridge/intercept"
	"github.com/coder/aibridge/mcp"
	"github.com/coder/aibridge/recorder"
	"github.com/coder/aibridge/tracing"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"promptgate/backend/internal/platform/proxylimits"
)

const unknownEmbeddingModel = "coder-aibridge-unknown"

// embeddingInterceptor forwards OpenAI-compatible embedding requests and records token usage.
type embeddingInterceptor struct {
	id                       uuid.UUID
	provider                 coderbridge.Provider
	rawBody                  []byte
	model                    string
	tracer                   trace.Tracer
	logger                   cdrslog.Logger
	recorder                 recorder.Recorder
	credential               intercept.CredentialInfo
	httpClient               *http.Client
	maxBufferedResponseBytes int64
}

// newEmbeddingInterceptor creates an interceptor for a single OpenAI-compatible embeddings request.
func newEmbeddingInterceptor(provider coderbridge.Provider, rawBody []byte, tracer trace.Tracer, credential string, opts providerRuntimeOptions) *embeddingInterceptor {
	opts = normalizeProviderRuntimeOptions(opts)
	return &embeddingInterceptor{
		id:                       uuid.New(),
		provider:                 provider,
		rawBody:                  rawBody,
		model:                    embeddingModel(rawBody),
		tracer:                   tracer,
		credential:               intercept.NewCredentialInfo(intercept.CredentialKindCentralized, credential),
		httpClient:               opts.httpClient,
		maxBufferedResponseBytes: opts.maxBufferedResponseBytes,
	}
}

// ID returns the unique interception ID.
func (i *embeddingInterceptor) ID() uuid.UUID {
	return i.id
}

// Setup attaches logging and recording dependencies supplied by aibridge.
func (i *embeddingInterceptor) Setup(logger cdrslog.Logger, rec recorder.Recorder, _ mcp.ServerProxier) {
	i.logger = logger.Named("embeddings")
	i.recorder = rec
}

// Model returns the requested embedding model or a stable unknown value.
func (i *embeddingInterceptor) Model() string {
	if strings.TrimSpace(i.model) == "" {
		return unknownEmbeddingModel
	}
	return i.model
}

// ProcessRequest forwards the embeddings request upstream and records token usage from the response.
func (i *embeddingInterceptor) ProcessRequest(w http.ResponseWriter, r *http.Request) (outErr error) {
	ctx, span := i.tracer.Start(r.Context(), "Intercept.ProcessRequest", trace.WithAttributes(tracing.InterceptionAttributesFromContext(r.Context())...))
	defer tracing.EndSpanErr(span, &outErr)

	upstreamURL, err := embeddingUpstreamURL(i.provider.BaseURL(), r.URL.RawQuery)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "request error", http.StatusBadGateway)
		return nil
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL, bytes.NewReader(i.rawBody))
	if err != nil {
		return fmt.Errorf("create upstream embeddings request: %w", err)
	}
	upstreamReq.Header = intercept.PrepareClientHeaders(r.Header)
	i.provider.InjectAuthHeader(&upstreamReq.Header)

	resp, err := i.httpClient.Do(upstreamReq)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		i.logger.Warn(ctx, "embeddings upstream request failed", cdrslog.Error(err))
		http.Error(w, "upstream proxy error", http.StatusBadGateway)
		return nil
	}
	defer resp.Body.Close()

	body, err := proxylimits.ReadAll(resp.Body, i.maxBufferedResponseBytes)
	if err != nil {
		if errors.Is(err, proxylimits.ErrExceeded) {
			span.SetStatus(codes.Error, err.Error())
			i.logger.Warn(ctx, "embeddings upstream response exceeded buffer limit", cdrslog.F("limit_bytes", i.maxBufferedResponseBytes))
			http.Error(w, "response_body_too_large", http.StatusBadGateway)
			return nil
		}
		return fmt.Errorf("read embeddings upstream response: %w", err)
	}

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		i.recordTokenUsage(ctx, body)
	}
	return nil
}

// Streaming reports that embeddings responses are handled as buffered responses.
func (*embeddingInterceptor) Streaming() bool {
	return false
}

// TraceAttributes returns tracing attributes for the embeddings request.
func (i *embeddingInterceptor) TraceAttributes(r *http.Request) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(tracing.RequestPath, r.URL.Path),
		attribute.String(tracing.InterceptionID, i.id.String()),
		attribute.String(tracing.InitiatorID, aibcontext.ActorIDFromContext(r.Context())),
		attribute.String(tracing.Provider, i.provider.Name()),
		attribute.String(tracing.Model, i.Model()),
		attribute.Bool(tracing.Streaming, false),
	}
}

// Credential returns metadata describing the centralized upstream credential.
func (i *embeddingInterceptor) Credential() intercept.CredentialInfo {
	return i.credential
}

// CorrelatingToolCallID returns nil because embeddings are not tool-call correlated.
func (*embeddingInterceptor) CorrelatingToolCallID() *string {
	return nil
}
