package proxy

import (
	"context"
	"testing"

	aibrecorder "github.com/coder/aibridge/recorder"
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
		"promptgate.identity.email": "person@example.com", "llm.token_count.prompt": int64(20),
		"llm.token_count.completion": int64(5), "llm.token_count.total": int64(25),
		"llm.token_count.completion_details.reasoning": int64(2),
	} {
		if got := attrs[key]; got != want {
			t.Errorf("attribute %s: got %#v, want %#v", key, got, want)
		}
	}
}

// TestTelemetryRecorderMapsNativeSession verifies native metadata takes priority over AIBridge.
func TestTelemetryRecorderMapsNativeSession(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	ctx := proxyruntime.WithNativeSession(context.Background(), proxyruntime.NativeSession{
		SessionID: "native-session", SessionSource: "x-openwebui-chat-id",
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
		"promptgate.session.source":    "x-openwebui-chat-id",
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
