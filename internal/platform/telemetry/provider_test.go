package telemetry

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"promptgate/backend/internal/platform/config"

	"go.opentelemetry.io/otel/attribute"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/protobuf/proto"
)

// TestProviderExportsAuthenticatedPhoenixBatch verifies the OTLP wire contract and project routing.
func TestProviderExportsAuthenticatedPhoenixBatch(t *testing.T) {
	received := make(chan *collectortrace.ExportTraceServiceRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer phoenix-secret" {
			t.Errorf("unexpected authorization header %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Project-Name") != "platform-analysis" {
			t.Errorf("unexpected project header %q", r.Header.Get("X-Project-Name"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		request := &collectortrace.ExportTraceServiceRequest{}
		if err := proto.Unmarshal(body, request); err != nil {
			t.Errorf("decode OTLP request: %v", err)
			return
		}
		received <- request
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider, err := NewProvider(context.Background(), config.OTelConfig{
		Enabled: true, Endpoint: server.URL, APIKey: "phoenix-secret", ProjectName: "platform-analysis",
		ServiceName: "promptgate-proxy", Environment: "test", Insecure: true,
		ExportTimeout: time.Second, BatchTimeout: time.Millisecond, MaxQueueSize: 10, MaxExportBatchSize: 10,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	_, span := provider.tracerProvider.Tracer("test").Start(context.Background(), "llm")
	span.SetAttributes(
		attribute.String("openinference.span.kind", "LLM"),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.Int64("gen_ai.usage.input_tokens", 12),
		attribute.Bool("gen_ai.request.stream", true),
		attribute.String("session.id", "native-session"),
		attribute.String("gen_ai.conversation.id", "native-session"),
		attribute.String("promptgate.session.source", "x-session-id"),
	)
	span.End()
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown provider: %v", err)
	}

	select {
	case request := <-received:
		if len(request.ResourceSpans) != 1 || len(request.ResourceSpans[0].ScopeSpans) != 1 {
			t.Fatalf("unexpected OTLP payload: %#v", request)
		}
		spans := request.ResourceSpans[0].ScopeSpans[0].Spans
		if len(spans) != 1 {
			t.Fatalf("expected one exported span, got %d", len(spans))
		}
		resourceAttributes := request.ResourceSpans[0].Resource.Attributes
		resources := make(map[string]string, len(resourceAttributes))
		for _, attr := range resourceAttributes {
			resources[attr.Key] = attr.Value.GetStringValue()
		}
		for key, want := range map[string]string{
			"service.name": "promptgate-proxy", "deployment.environment.name": "test", "openinference.project.name": "platform-analysis",
		} {
			if got := resources[key]; got != want {
				t.Errorf("resource attribute %s: got %q, want %q", key, got, want)
			}
		}
		attributes := make(map[string]any, len(spans[0].Attributes))
		for _, attr := range spans[0].Attributes {
			switch value := attr.Value.Value.(type) {
			case *commonv1.AnyValue_StringValue:
				attributes[attr.Key] = value.StringValue
			case *commonv1.AnyValue_IntValue:
				attributes[attr.Key] = value.IntValue
			case *commonv1.AnyValue_BoolValue:
				attributes[attr.Key] = value.BoolValue
			}
		}
		for key, want := range map[string]any{
			"openinference.span.kind":   "LLM",
			"gen_ai.operation.name":     "chat",
			"gen_ai.usage.input_tokens": int64(12),
			"gen_ai.request.stream":     true,
			"session.id":                "native-session",
			"gen_ai.conversation.id":    "native-session",
			"promptgate.session.source": "x-session-id",
		} {
			if got := attributes[key]; got != want {
				t.Errorf("exported attribute %s: got %#v, want %#v", key, got, want)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for OTLP export")
	}
}

// TestProviderDisabledIsNoop verifies disabled telemetry needs no endpoint.
func TestProviderDisabledIsNoop(t *testing.T) {
	provider, err := NewProvider(context.Background(), config.OTelConfig{}, slog.Default())
	if err != nil {
		t.Fatalf("disabled provider: %v", err)
	}
	if provider.tracerProvider != nil {
		t.Fatal("disabled provider must not install an SDK tracer provider")
	}
}
