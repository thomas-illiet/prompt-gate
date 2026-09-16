package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	aibrecorder "github.com/coder/aibridge/recorder"
	aibtracing "github.com/coder/aibridge/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	proxyruntime "promptgate/backend/internal/runtime/proxy"
)

type telemetryInterception struct {
	span                                 trace.Span
	mu                                   sync.Mutex
	input, output, cacheRead, cacheWrite int64
	extra                                map[string]int64
}

// TelemetryRecorder enriches AIBridge interception spans and delegates persistence.
type TelemetryRecorder struct {
	inner          aibrecorder.Recorder
	capturePrompts bool
	costEnabled    bool
	inputRate      float64
	outputRate     float64
	embeddingRate  float64
	interceptions  sync.Map
}

// NewTelemetryRecorder decorates a usage recorder with OpenInference and OTel GenAI attributes.
func NewTelemetryRecorder(inner aibrecorder.Recorder, capturePrompts bool) *TelemetryRecorder {
	return &TelemetryRecorder{inner: inner, capturePrompts: capturePrompts}
}

// WithEstimatedCosts enables estimated USD attributes using per-million-token rates.
func (r *TelemetryRecorder) WithEstimatedCosts(input, output, embedding float64) *TelemetryRecorder {
	r.costEnabled = true
	r.inputRate, r.outputRate, r.embeddingRate = input, output, embedding
	return r
}

func (r *TelemetryRecorder) RecordInterception(ctx context.Context, req *aibrecorder.InterceptionRecord) error {
	if req != nil {
		span := trace.SpanFromContext(ctx)
		attrs := append(interceptionTelemetryAttributes(req.Provider, req.Model, aibtracing.InterceptionAttributesFromContext(ctx)),
			attribute.String("promptgate.provider.name", req.ProviderName),
			attribute.String("promptgate.interception.id", req.ID),
			attribute.String("user.id", req.InitiatorID),
			attribute.String("promptgate.client", req.Client),
			attribute.String("promptgate.user_agent", req.UserAgent),
			attribute.String("promptgate.credential.kind", req.CredentialKind),
		)
		if session, ok := proxyruntime.NativeSessionFromContext(ctx); ok {
			if session.SessionID != "" {
				attrs = append(attrs,
					attribute.String("session.id", session.SessionID),
					attribute.String(genAIConversationID, session.SessionID),
					attribute.String("promptgate.session.source", session.SessionSource),
				)
			}
			if session.ParentSessionID != "" {
				attrs = append(attrs, attribute.String("promptgate.parent_session.id", session.ParentSessionID))
			}
			if session.MessageID != "" {
				attrs = append(attrs, attribute.String("promptgate.message.id", session.MessageID))
			}
		} else if req.ClientSessionID != nil {
			attrs = append(attrs,
				attribute.String("session.id", *req.ClientSessionID),
				attribute.String(genAIConversationID, *req.ClientSessionID),
				attribute.String("promptgate.session.source", "aibridge"),
			)
		}
		if req.CorrelatingToolCallID != nil {
			attrs = append(attrs, attribute.String("promptgate.correlating_tool_call.id", *req.CorrelatingToolCallID))
		}
		for key, value := range req.Metadata {
			if text, ok := value.(string); ok && text != "" {
				attrs = append(attrs, attribute.String("promptgate.identity."+key, text))
			}
		}
		span.SetAttributes(attrs...)
		proxyruntime.BindResponseTiming(ctx, span)
		proxyruntime.BindOutputSpan(ctx, span)
		r.interceptions.Store(req.ID, &telemetryInterception{span: span, extra: make(map[string]int64)})
	}
	return r.inner.RecordInterception(ctx, req)
}

func (r *TelemetryRecorder) RecordInterceptionEnded(ctx context.Context, req *aibrecorder.InterceptionRecordEnded) error {
	if req != nil {
		if _, ok := r.load(req.ID); ok {
			proxyruntime.PublishOutput(ctx)
			time.AfterFunc(5*time.Minute, func() { r.interceptions.Delete(req.ID) })
		}
	}
	return r.inner.RecordInterceptionEnded(ctx, req)
}

func (r *TelemetryRecorder) RecordPromptUsage(ctx context.Context, req *aibrecorder.PromptUsageRecord) error {
	if r.capturePrompts && req != nil {
		if state, ok := r.load(req.InterceptionID); ok {
			state.span.SetAttributes(
				attribute.String(openInferenceInputValue, req.Prompt),
				attribute.String(openInferenceInputMIMEType, "text/plain"),
			)
		}
	}
	return r.inner.RecordPromptUsage(ctx, req)
}

func (r *TelemetryRecorder) RecordTokenUsage(ctx context.Context, req *aibrecorder.TokenUsageRecord) error {
	if req != nil {
		if state, ok := r.load(req.InterceptionID); ok {
			state.mu.Lock()
			state.input += req.Input
			state.output += req.Output
			state.cacheRead += req.CacheReadInputTokens
			state.cacheWrite += req.CacheWriteInputTokens
			for key, value := range req.ExtraTokenTypes {
				state.extra[key] += value
			}
			promptTokens := state.input + state.cacheRead + state.cacheWrite
			reasoningTokens := state.extra["completion_reasoning"]
			attrs := tokenTelemetryAttributes(promptTokens, state.output, state.cacheRead, state.cacheWrite, reasoningTokens)
			for key, value := range state.extra {
				attrs = append(attrs, attribute.Int64("promptgate.token_count."+key, value))
			}
			if value := state.extra["prompt_audio"]; value > 0 {
				attrs = append(attrs, attribute.Int64("llm.token_count.prompt_details.audio", value))
			}
			if value := state.extra["completion_audio"]; value > 0 {
				attrs = append(attrs, attribute.Int64("llm.token_count.completion_details.audio", value))
			}
			state.span.SetAttributes(attrs...)
			isEmbedding := metadataTokenUsageType(req.Metadata) == tokenUsageTypeEmbedding
			if isEmbedding {
				state.span.SetAttributes(
					attribute.String(openInferenceSpanKind, openInferenceSpanKindEmbedding),
					attribute.String(genAIOperationName, genAIOperationEmbeddings),
				)
			}
			if r.costEnabled {
				inputCost := float64(promptTokens) * r.inputRate / usageCostTokenUnit
				outputCost := float64(state.output) * r.outputRate / usageCostTokenUnit
				if isEmbedding {
					inputCost = float64(promptTokens) * r.embeddingRate / usageCostTokenUnit
					outputCost = 0
				}
				state.span.SetAttributes(
					attribute.Float64("llm.cost.prompt", inputCost),
					attribute.Float64("llm.cost.completion", outputCost),
					attribute.Float64("llm.cost.total", inputCost+outputCost),
					attribute.Bool("promptgate.cost.estimated", true),
				)
			}
			state.mu.Unlock()
		}
	}
	return r.inner.RecordTokenUsage(ctx, req)
}

func (r *TelemetryRecorder) RecordToolUsage(ctx context.Context, req *aibrecorder.ToolUsageRecord) error {
	if req != nil {
		if state, ok := r.load(req.InterceptionID); ok {
			args, _ := json.Marshal(req.Args)
			attrs := []attribute.KeyValue{
				attribute.String("tool.name", req.Tool),
				attribute.String("tool.call.id", req.ToolCallID),
				attribute.String(openInferenceInputValue, string(args)),
				attribute.String(openInferenceInputMIMEType, "application/json"),
				attribute.Bool("promptgate.tool.injected", req.Injected),
			}
			if req.ServerURL != nil {
				attrs = append(attrs, attribute.String("promptgate.tool.server_url", *req.ServerURL))
			}
			if req.InvocationError != nil {
				attrs = append(attrs,
					attribute.String(standardErrorType, toolInvocationErrorType),
					attribute.String(standardErrorMessage, req.InvocationError.Error()),
				)
			}
			state.span.AddEvent(fmt.Sprintf("tool:%s", req.Tool), trace.WithAttributes(attrs...))
		}
	}
	return r.inner.RecordToolUsage(ctx, req)
}

func (r *TelemetryRecorder) RecordModelThought(ctx context.Context, req *aibrecorder.ModelThoughtRecord) error {
	return r.inner.RecordModelThought(ctx, req)
}

func (r *TelemetryRecorder) load(id string) (*telemetryInterception, bool) {
	value, ok := r.interceptions.Load(id)
	if !ok {
		return nil, false
	}
	state, ok := value.(*telemetryInterception)
	return state, ok
}
