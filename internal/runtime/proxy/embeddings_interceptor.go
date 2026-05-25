package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
)

const unknownEmbeddingModel = "coder-aibridge-unknown"

const (
	embeddingTokenSourceProviderUsage   = "provider_usage"
	embeddingTokenSourceProviderMissing = "provider_usage_missing"
	embeddingTokenWarningTotalMismatch  = "embedding_total_tokens_mismatch"
)

var responseHeadersExcludedFromCopy = map[string]struct{}{
	"Connection":          {},
	"Content-Length":      {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

// embeddingInterceptor forwards OpenAI-compatible embedding requests and records token usage.
type embeddingInterceptor struct {
	id         uuid.UUID
	provider   coderbridge.Provider
	rawBody    []byte
	model      string
	tracer     trace.Tracer
	logger     cdrslog.Logger
	recorder   recorder.Recorder
	credential intercept.CredentialInfo
}

func newEmbeddingInterceptor(provider coderbridge.Provider, rawBody []byte, tracer trace.Tracer, credential string) *embeddingInterceptor {
	return &embeddingInterceptor{
		id:         uuid.New(),
		provider:   provider,
		rawBody:    rawBody,
		model:      embeddingModel(rawBody),
		tracer:     tracer,
		credential: intercept.NewCredentialInfo(intercept.CredentialKindCentralized, credential),
	}
}

func (i *embeddingInterceptor) ID() uuid.UUID {
	return i.id
}

func (i *embeddingInterceptor) Setup(logger cdrslog.Logger, rec recorder.Recorder, _ mcp.ServerProxier) {
	i.logger = logger.Named("embeddings")
	i.recorder = rec
}

func (i *embeddingInterceptor) Model() string {
	if strings.TrimSpace(i.model) == "" {
		return unknownEmbeddingModel
	}
	return i.model
}

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

	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		i.logger.Warn(ctx, "embeddings upstream request failed", cdrslog.Error(err))
		http.Error(w, "upstream proxy error", http.StatusBadGateway)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
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

func (*embeddingInterceptor) Streaming() bool {
	return false
}

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

func (i *embeddingInterceptor) Credential() intercept.CredentialInfo {
	return i.credential
}

func (*embeddingInterceptor) CorrelatingToolCallID() *string {
	return nil
}

func (i *embeddingInterceptor) recordTokenUsage(ctx context.Context, body []byte) {
	if i.recorder == nil {
		return
	}

	responseID := i.id.String()
	metadata := recorder.Metadata{
		"type": "embedding",
	}

	var payload embeddingResponsePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		i.logger.Warn(ctx, "failed to decode embeddings usage", cdrslog.Error(err), cdrslog.F("interception_id", i.id.String()))
	} else {
		if id := strings.TrimSpace(payload.ID); id != "" {
			responseID = id
		}
		inputTokens, warning := embeddingProviderInputTokens(payload.Usage)
		if inputTokens > 0 {
			metadata["token_source"] = embeddingTokenSourceProviderUsage
			if warning != "" {
				metadata["token_warning"] = warning
				i.logger.Warn(ctx, "embeddings provider usage mismatch", cdrslog.F("interception_id", i.id.String()), cdrslog.F("model", i.Model()), cdrslog.F("token_warning", warning))
			}
			i.recordEmbeddingTokens(ctx, responseID, inputTokens, metadata)
			return
		}
	}

	metadata["token_source"] = embeddingTokenSourceProviderMissing
	i.recordEmbeddingTokens(ctx, responseID, 0, metadata)
}

func (i *embeddingInterceptor) recordEmbeddingTokens(ctx context.Context, responseID string, inputTokens int64, metadata recorder.Metadata) {
	_ = i.recorder.RecordTokenUsage(ctx, &recorder.TokenUsageRecord{
		InterceptionID: i.id.String(),
		MsgID:          responseID,
		Input:          inputTokens,
		Output:         0,
		Metadata:       metadata,
	})
}

type embeddingResponsePayload struct {
	ID    string                 `json:"id"`
	Usage embeddingResponseUsage `json:"usage"`
}

type embeddingResponseUsage struct {
	PromptTokens *int64 `json:"prompt_tokens"`
	InputTokens  *int64 `json:"input_tokens"`
	TotalTokens  *int64 `json:"total_tokens"`
}

func embeddingProviderInputTokens(usage embeddingResponseUsage) (int64, string) {
	inputTokens := int64(0)
	switch {
	case usage.PromptTokens != nil && *usage.PromptTokens > 0:
		inputTokens = *usage.PromptTokens
	case usage.InputTokens != nil && *usage.InputTokens > 0:
		inputTokens = *usage.InputTokens
	case usage.PromptTokens == nil && usage.InputTokens == nil && usage.TotalTokens != nil && *usage.TotalTokens > 0:
		inputTokens = *usage.TotalTokens
	}

	if inputTokens > 0 && usage.TotalTokens != nil && *usage.TotalTokens > 0 && *usage.TotalTokens != inputTokens {
		return inputTokens, embeddingTokenWarningTotalMismatch
	}
	return inputTokens, ""
}

func embeddingModel(raw []byte) string {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return unknownEmbeddingModel
	}
	if strings.TrimSpace(payload.Model) == "" {
		return unknownEmbeddingModel
	}
	return payload.Model
}

func embeddingUpstreamURL(baseURL string, rawQuery string) (string, error) {
	upstream, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return "", fmt.Errorf("parse embeddings upstream URL: %w", err)
	}
	requestPath, err := url.JoinPath(upstream.Path, routeEmbeddings)
	if err != nil {
		return "", fmt.Errorf("join embeddings upstream path: %w", err)
	}
	if requestPath == "" || requestPath[0] != '/' {
		requestPath = "/" + requestPath
	}
	upstream.Path = requestPath
	upstream.RawPath = ""
	upstream.RawQuery = rawQuery
	return upstream.String(), nil
}

func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		if _, excluded := responseHeadersExcludedFromCopy[http.CanonicalHeaderKey(key)]; excluded {
			continue
		}
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
