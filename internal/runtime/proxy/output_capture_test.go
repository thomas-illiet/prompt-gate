package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOutputCaptureMiddlewarePublishesBlockingOutputOnBoundSpan(t *testing.T) {
	body := `{"choices":[{"message":{"content":"hello world"}}]}`
	got, attrs := captureHandlerOutput(t, "application/json", []string{body}, int64(len(body)), http.StatusOK, true, false)
	if got != body {
		t.Fatalf("client response changed: got %q, want %q", got, body)
	}
	if attrs[openInferenceOutputValue] != "hello world" || attrs[openInferenceOutputMIMEType] != "text/plain" {
		t.Fatalf("unexpected output attributes: %#v", attrs)
	}
}

func TestOutputCaptureMiddlewarePublishesCompleteStreamingOutput(t *testing.T) {
	chunks := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"chat \"}}]}\n\n",
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"response \"}\n\n",
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"message\"}}\n\n",
		"data: [DONE]\n\n",
	}
	_, attrs := captureHandlerOutput(t, "text/event-stream", chunks, 4096, http.StatusOK, true, false)
	if attrs[openInferenceOutputValue] != "chat response message" {
		t.Fatalf("unexpected streaming output: %#v", attrs)
	}
}

func TestOutputCaptureMiddlewarePublishesChatCompletionsThinkingWithoutOutputCapture(t *testing.T) {
	body := `{"choices":[{"message":{"content":"visible answer","reasoning_content":"provider reasoning"}}]}`
	_, attrs := captureHandlerOutput(t, "application/json", []string{body}, int64(len(body)), http.StatusOK, false, true)
	if _, ok := attrs[openInferenceOutputValue]; ok {
		t.Fatalf("output must remain disabled: %#v", attrs)
	}
	for key, want := range map[string]any{
		"llm.output_messages.0.message.role":                            "assistant",
		"llm.output_messages.0.message.contents.0.message_content.type": "reasoning",
		"llm.output_messages.0.message.contents.0.message_content.text": "provider reasoning",
		"promptgate.model_thoughts.0.source":                            "chat_completions",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("attribute %s: got %#v, want %#v", key, got, want)
		}
	}
}

func TestOutputCaptureMiddlewarePublishesStreamingChatCompletionsThinking(t *testing.T) {
	chunks := []string{
		"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"first \"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"second\",\"content\":\"answer\"}}]}\n\n",
		"data: [DONE]\n\n",
	}
	_, attrs := captureHandlerOutput(t, "text/event-stream", chunks, 4096, http.StatusOK, true, true)
	if attrs[openInferenceOutputValue] != "answer" {
		t.Fatalf("unexpected output: %#v", attrs)
	}
	if attrs["llm.output_messages.0.message.contents.0.message_content.text"] != "first second" {
		t.Fatalf("unexpected thinking: %#v", attrs)
	}
}

func TestOutputCaptureMiddlewareOmitsUnsafeResponses(t *testing.T) {
	body := `{"choices":[{"message":{"content":"secret"}}]}`
	tests := []struct {
		name       string
		maxBytes   int64
		statusCode int
	}{
		{"oversized", int64(len(body) - 1), http.StatusOK},
		{"provider error", int64(len(body)), http.StatusBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, attrs := captureHandlerOutput(t, "application/json", []string{body}, tt.maxBytes, tt.statusCode, true, true)
			if _, ok := attrs[openInferenceOutputValue]; ok {
				t.Fatalf("output must be omitted: %#v", attrs)
			}
		})
	}
}

func captureHandlerOutput(t *testing.T, contentType string, chunks []string, maxBytes int64, statusCode int, captureOutput, captureThinking bool) (string, map[string]any) {
	t.Helper()
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	handler := OutputCaptureMiddleware(maxBytes, captureOutput, captureThinking)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, span := provider.Tracer("test").Start(r.Context(), "interception")
		BindOutputSpan(r.Context(), span)
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(statusCode)
		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		PublishOutput(context.WithoutCancel(r.Context()))
		span.End()
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", nil))
	attrs := make(map[string]any)
	for _, attr := range spans.Ended()[0].Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsInterface()
	}
	return recorder.Body.String(), attrs
}

func TestExtractAssistantOutputFormats(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"responses", `{"output":[{"type":"message","content":[{"type":"output_text","text":"hello "},{"type":"output_text","text":"world"}]}]}`, "hello world"},
		{"messages", `{"content":[{"type":"thinking","thinking":"private"},{"type":"text","text":"hello world"}]}`, "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractAssistantOutput([]byte(tt.body), "application/json"); got != tt.want {
				t.Fatalf("output: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOutputCaptureResponseWriterImplementsStreamingInterfaces(t *testing.T) {
	writer := &outputCaptureResponseWriter{ResponseWriter: httptest.NewRecorder(), capture: &outputCapture{maxBytes: 10}}
	if _, ok := any(writer).(http.Flusher); !ok {
		t.Fatal("output capture writer must preserve http.Flusher")
	}
	if _, ok := any(writer).(interface{ Unwrap() http.ResponseWriter }); !ok {
		t.Fatal("output capture writer must support ResponseController unwrapping")
	}
	if _, err := writer.ReadFrom(strings.NewReader("payload")); err != nil {
		t.Fatalf("read from: %v", err)
	}
}
