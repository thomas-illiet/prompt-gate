package runtime

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOutputCaptureTransportCapturesBlockingResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"chat completions", `{"choices":[{"message":{"content":"hello world"}}]}`, "hello world"},
		{"responses", `{"output":[{"type":"message","content":[{"type":"output_text","text":"hello "},{"type":"output_text","text":"world"}]}]}`, "hello world"},
		{"messages", `{"content":[{"type":"thinking","thinking":"private"},{"type":"text","text":"hello world"}]}`, "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capturedOutput(t, "application/json", tt.body, int64(len(tt.body)))
			if got != tt.want {
				t.Fatalf("output: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOutputCaptureTransportCapturesStreamingResponses(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"chat "}}]}`,
		`data: {"type":"response.output_text.delta","delta":"response "}`,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"message"}}`,
		`data: [DONE]`,
	}, "\n\n")
	if got := capturedOutput(t, "text/event-stream", body, int64(len(body))); got != "chat response message" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestOutputCaptureTransportOmitsOversizedResponse(t *testing.T) {
	body := `{"choices":[{"message":{"content":"secret"}}]}`
	if got := capturedOutput(t, "application/json", body, int64(len(body)-1)); got != "" {
		t.Fatalf("oversized output must be omitted, got %q", got)
	}
}

func capturedOutput(t *testing.T, contentType, body string, maxBytes int64) string {
	t.Helper()
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	ctx, span := provider.Tracer("test").Start(context.Background(), "provider")
	transport := NewOutputCaptureTransport(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{contentType}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}), maxBytes)
	resp, err := transport.RoundTrip((&http.Request{Method: http.MethodPost}).WithContext(ctx))
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read response: %v", err)
	}
	_ = resp.Body.Close()
	span.End()
	attrs := make(map[string]any)
	for _, attr := range spans.Ended()[0].Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsInterface()
	}
	value, _ := attrs[openInferenceOutputValue].(string)
	if value != "" && attrs[openInferenceOutputMIMEType] != "text/plain" {
		t.Fatalf("unexpected output MIME type: %#v", attrs[openInferenceOutputMIMEType])
	}
	return value
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
