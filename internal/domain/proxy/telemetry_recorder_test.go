package proxy

import (
	"context"
	"errors"
	"math"
	"testing"

	aibrecorder "github.com/coder/aibridge/recorder"
	aibtracing "github.com/coder/aibridge/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	proxyruntime "promptgate/backend/internal/runtime/proxy"
)

// TestTelemetryRecorderMapsOpenInferenceAttributes verifies identity, prompts, and cumulative tokens.
func TestTelemetryRecorderMapsOpenInferenceAttributes(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	ctx, span := provider.Tracer("test").Start(context.Background(), "interception")
	recorder := NewTelemetryRecorder(noopTelemetryRecorder{}, true)
	id := "interception-id"
	if err := recorder.RecordInterception(ctx, &aibrecorder.InterceptionRecord{
		ID: id, InitiatorID: "user-id", Provider: "openai", ProviderName: "central", Model: "gpt-test",
		Metadata: aibrecorder.Metadata{"email": "person@example.com", "credentialId": "key-id"},
	}); err != nil {
		t.Fatalf("record interception: %v", err)
	}
	_ = recorder.RecordPromptUsage(ctx, &aibrecorder.PromptUsageRecord{InterceptionID: id, Prompt: "raw prompt"})
	_ = recorder.RecordTokenUsage(ctx, &aibrecorder.TokenUsageRecord{InterceptionID: id, Input: 10, Output: 4, CacheReadInputTokens: 3, ExtraTokenTypes: map[string]int64{"completion_reasoning": 2}})
	_ = recorder.RecordTokenUsage(ctx, &aibrecorder.TokenUsageRecord{InterceptionID: id, Input: 5, Output: 1, CacheWriteInputTokens: 2})
	span.End()

	ended := spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected one span, got %d", len(ended))
	}
	attrs := spanAttributes(ended[0])
	for key, want := range map[string]any{
		"openinference.span.kind": "LLM", "input.value": "raw prompt", "user.id": "user-id",
		"gen_ai.provider.name": "openai", "gen_ai.request.model": "gpt-test", "gen_ai.operation.name": "chat",
		"promptgate.identity.email": "person@example.com", "llm.token_count.prompt": int64(20),
		"llm.token_count.completion": int64(5), "llm.token_count.total": int64(25),
		"llm.token_count.completion_details.reasoning": int64(2),
		"gen_ai.usage.input_tokens":                    int64(20), "gen_ai.usage.output_tokens": int64(5),
		"gen_ai.usage.cache_read.input_tokens": int64(3), "gen_ai.usage.cache_creation.input_tokens": int64(2),
		"gen_ai.usage.cache_write.input_tokens": int64(2), "gen_ai.usage.reasoning.output_tokens": int64(2),
	} {
		if got := attrs[key]; got != want {
			t.Errorf("attribute %s: got %#v, want %#v", key, got, want)
		}
	}
}

func TestTelemetryRecorderClassifiesEmbeddingsWithoutCosts(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	bridgeAttrs := []attribute.KeyValue{
		attribute.String(aibtracing.RequestPath, "/openai/v1/embeddings"),
		attribute.String(aibtracing.Provider, "configured-provider"),
		attribute.String(aibtracing.Model, "text-embedding-3-small"),
		attribute.String(aibtracing.InitiatorID, "bridge-user"),
		attribute.Bool(aibtracing.Streaming, false),
	}
	ctx := aibtracing.WithInterceptionAttributesInContext(context.Background(), bridgeAttrs)
	ctx, span := provider.Tracer("test").Start(ctx, "interception")
	recorder := NewTelemetryRecorder(noopTelemetryRecorder{}, false)
	_ = recorder.RecordInterception(ctx, &aibrecorder.InterceptionRecord{
		ID: "embedding-id", Provider: "openai", ProviderName: "configured-provider", Model: "text-embedding-3-small", InitiatorID: "bridge-user",
	})
	_ = recorder.RecordTokenUsage(ctx, &aibrecorder.TokenUsageRecord{
		InterceptionID: "embedding-id", Input: 7, CacheReadInputTokens: 2,
		Metadata: aibrecorder.Metadata{"type": tokenUsageTypeEmbedding},
	})
	span.End()
	attrs := spanAttributes(spans.Ended()[0])
	for key, want := range map[string]any{
		"openinference.span.kind": "EMBEDDING", "gen_ai.operation.name": "embeddings",
		"gen_ai.provider.name": "openai", "gen_ai.request.model": "text-embedding-3-small",
		"url.path": "/openai/v1/embeddings", "user.id": "bridge-user", "gen_ai.usage.input_tokens": int64(9),
	} {
		if got := attrs[key]; got != want {
			t.Errorf("attribute %s: got %#v, want %#v", key, got, want)
		}
	}
	if _, exists := attrs["llm.cost.total"]; exists {
		t.Fatal("cost attributes must not be emitted when cost calculation is disabled")
	}
}

func TestTelemetryRecorderUsesEmbeddingCostRate(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	ctx, span := provider.Tracer("test").Start(context.Background(), "interception")
	recorder := NewTelemetryRecorder(noopTelemetryRecorder{}, false).WithEstimatedCosts(2, 4, 3)
	_ = recorder.RecordInterception(ctx, &aibrecorder.InterceptionRecord{ID: "id", Provider: "openai", Model: "embedding-model"})
	_ = recorder.RecordTokenUsage(ctx, &aibrecorder.TokenUsageRecord{
		InterceptionID: "id", Input: 5, Output: 9,
		Metadata: aibrecorder.Metadata{"endpoint": tokenUsageEndpointEmbeddings},
	})
	span.End()
	attrs := spanAttributes(spans.Ended()[0])
	if got, want := attrs["llm.cost.total"].(float64), float64(15)/usageCostTokenUnit; math.Abs(got-want) > 1e-12 {
		t.Fatalf("embedding cost: got %#v, want %#v", got, want)
	}
	if attrs["llm.cost.completion"] != float64(0) || attrs[genAIOperationName] != genAIOperationEmbeddings {
		t.Fatalf("unexpected embedding attributes: %#v", attrs)
	}
}

func TestTelemetryRecorderMapsCostsAndToolErrors(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	ctx, span := provider.Tracer("test").Start(context.Background(), "interception")
	recorder := NewTelemetryRecorder(noopTelemetryRecorder{}, true).WithEstimatedCosts(2, 4, 3)
	_ = recorder.RecordInterception(ctx, &aibrecorder.InterceptionRecord{ID: "id", Provider: "openai", Model: "gpt-test"})
	_ = recorder.RecordTokenUsage(ctx, &aibrecorder.TokenUsageRecord{InterceptionID: "id", Input: 1, Output: 2})
	_ = recorder.RecordToolUsage(ctx, &aibrecorder.ToolUsageRecord{
		InterceptionID: "id", Tool: "lookup", ToolCallID: "call-1", Args: map[string]any{"email": "person@example.com"},
		InvocationError: errors.New("remote system exposed a detailed failure"),
	})
	span.End()
	ended := spans.Ended()[0]
	attrs := spanAttributes(ended)
	if got, want := attrs["llm.cost.total"].(float64), float64(10)/usageCostTokenUnit; math.Abs(got-want) > 1e-12 {
		t.Fatalf("llm.cost.total: got %#v, want %#v", got, want)
	}
	if got := attrs["promptgate.cost.estimated"]; got != true {
		t.Fatalf("promptgate.cost.estimated: got %#v, want true", got)
	}
	if ended.Status().Code != codes.Unset {
		t.Fatalf("tool failure must not fail the LLM span, got %v", ended.Status().Code)
	}
	if len(ended.Events()) != 1 {
		t.Fatalf("expected one tool event, got %d", len(ended.Events()))
	}
	eventAttrs := make(map[string]any)
	for _, attr := range ended.Events()[0].Attributes {
		eventAttrs[string(attr.Key)] = attr.Value.AsInterface()
	}
	if eventAttrs[standardErrorType] != toolInvocationErrorType || eventAttrs[standardErrorMessage] != "remote system exposed a detailed failure" {
		t.Fatalf("unexpected tool error attributes: %#v", eventAttrs)
	}
	if eventAttrs[openInferenceInputValue] != `{"email":"person@example.com"}` {
		t.Fatalf("tool arguments must remain exported: %#v", eventAttrs[openInferenceInputValue])
	}
}

// TestTelemetryRecorderMapsNativeSession verifies native metadata takes priority over AIBridge.
func TestTelemetryRecorderMapsNativeSession(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	ctx := proxyruntime.WithNativeSession(context.Background(), proxyruntime.NativeSession{
		SessionID: "native-session", SessionSource: "x-session-affinity",
		ParentSessionID: "parent-session", MessageID: "message-id",
	})
	ctx, span := provider.Tracer("test").Start(ctx, "interception")
	fallback := "aibridge-session"
	recorder := NewTelemetryRecorder(noopTelemetryRecorder{}, false)
	if err := recorder.RecordInterception(ctx, &aibrecorder.InterceptionRecord{
		ID: "id", ClientSessionID: &fallback,
	}); err != nil {
		t.Fatalf("record interception: %v", err)
	}
	span.End()
	attrs := spanAttributes(spans.Ended()[0])
	for key, want := range map[string]any{
		"session.id":                   "native-session",
		"gen_ai.conversation.id":       "native-session",
		"promptgate.session.source":    "x-session-affinity",
		"promptgate.parent_session.id": "parent-session",
		"promptgate.message.id":        "message-id",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("attribute %s: got %#v, want %#v", key, got, want)
		}
	}
}

// TestTelemetryRecorderUsesAIBridgeSessionFallback verifies body-derived sessions remain supported.
func TestTelemetryRecorderUsesAIBridgeSessionFallback(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	ctx, span := provider.Tracer("test").Start(context.Background(), "interception")
	sessionID := "body-derived-session"
	recorder := NewTelemetryRecorder(noopTelemetryRecorder{}, false)
	if err := recorder.RecordInterception(ctx, &aibrecorder.InterceptionRecord{
		ID: "id", ClientSessionID: &sessionID,
	}); err != nil {
		t.Fatalf("record interception: %v", err)
	}
	span.End()
	attrs := spanAttributes(spans.Ended()[0])
	if attrs["session.id"] != sessionID || attrs["gen_ai.conversation.id"] != sessionID {
		t.Fatalf("unexpected session attributes: %#v", attrs)
	}
	if attrs["promptgate.session.source"] != "aibridge" {
		t.Fatalf("unexpected session source: %#v", attrs["promptgate.session.source"])
	}
}

// TestTelemetryRecorderDoesNotCapturePromptByDefault verifies content consent is independent.
func TestTelemetryRecorderDoesNotCapturePromptByDefault(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	ctx, span := provider.Tracer("test").Start(context.Background(), "interception")
	recorder := NewTelemetryRecorder(noopTelemetryRecorder{}, false)
	_ = recorder.RecordInterception(ctx, &aibrecorder.InterceptionRecord{ID: "id"})
	_ = recorder.RecordPromptUsage(ctx, &aibrecorder.PromptUsageRecord{InterceptionID: "id", Prompt: "secret"})
	span.End()
	if _, exists := spanAttributes(spans.Ended()[0])["input.value"]; exists {
		t.Fatal("prompt must not be attached when capture is disabled")
	}
}

func spanAttributes(span sdktrace.ReadOnlySpan) map[string]any {
	values := make(map[string]any)
	for _, attr := range span.Attributes() {
		values[string(attr.Key)] = attr.Value.AsInterface()
	}
	return values
}

type noopTelemetryRecorder struct{}

func (noopTelemetryRecorder) RecordInterception(context.Context, *aibrecorder.InterceptionRecord) error {
	return nil
}
func (noopTelemetryRecorder) RecordInterceptionEnded(context.Context, *aibrecorder.InterceptionRecordEnded) error {
	return nil
}
func (noopTelemetryRecorder) RecordTokenUsage(context.Context, *aibrecorder.TokenUsageRecord) error {
	return nil
}
func (noopTelemetryRecorder) RecordPromptUsage(context.Context, *aibrecorder.PromptUsageRecord) error {
	return nil
}
func (noopTelemetryRecorder) RecordToolUsage(context.Context, *aibrecorder.ToolUsageRecord) error {
	return nil
}
func (noopTelemetryRecorder) RecordModelThought(context.Context, *aibrecorder.ModelThoughtRecord) error {
	return nil
}
