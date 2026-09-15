package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestResponseTimingMiddlewareRecordsFirstByte verifies streaming-safe latency observation.
func TestResponseTimingMiddlewareRecordsFirstByte(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	handler := ResponseTimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, span := provider.Tracer("test").Start(r.Context(), "interception")
		BindResponseTiming(r.Context(), span)
		_, _ = w.Write([]byte("ok"))
		span.End()
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Body.String() != "ok" {
		t.Fatalf("unexpected body %q", recorder.Body.String())
	}
	attrs := spans.Ended()[0].Attributes()
	for _, attr := range attrs {
		if string(attr.Key) == "promptgate.latency.time_to_first_byte_ms" {
			return
		}
	}
	t.Fatal("missing time-to-first-byte attribute")
}
