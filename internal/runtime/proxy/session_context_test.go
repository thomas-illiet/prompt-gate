package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestSessionContextMiddlewareRecognizesAllowlistedHeaders(t *testing.T) {
	tests := []struct {
		header string
		value  string
	}{
		{"x-claude-code-session-id", "claude"},
		{"x-openwebui-chat-id", "openwebui"},
		{"x-coder-chat-id", "coder"},
		{"x-kilocode-taskid", "kilo"},
		{"x-client-session-id", "copilot-cli"},
		{"x-interaction-id", "copilot-vscode"},
		{"x-mux-workspace-id", "mux"},
		{"x-session-id", "opencode"},
		{"session-id", "opencode-openai"},
		{"session_id", "codex"},
	}
	for _, test := range tests {
		t.Run(test.header, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/provider/v1/chat/completions", nil)
			req.Header.Set(test.header, "  "+test.value+"  ")
			got := captureNativeSession(t, req)
			if got.SessionID != test.value || got.SessionSource != test.header {
				t.Fatalf("got %#v, want ID %q from %q", got, test.value, test.header)
			}
		})
	}
}

func TestSessionContextMiddlewareUsesPriorityAndRelatedIDs(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/chat/completions", nil)
	req.Header.Set("X-Session-Id", "lower-priority")
	req.Header.Set("X-OpenWebUI-Chat-Id", "chat-id")
	req.Header.Set("X-Parent-Session-Id", "parent-id")
	req.Header.Set("X-OpenWebUI-Message-Id", "message-id")
	got := captureNativeSession(t, req)
	if got != (NativeSession{
		SessionID: "chat-id", SessionSource: "x-openwebui-chat-id",
		ParentSessionID: "parent-id", MessageID: "message-id",
	}) {
		t.Fatalf("unexpected session metadata: %#v", got)
	}
}

func TestSessionContextMiddlewareRejectsInvalidAndUnknownValues(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
	}{
		{"empty", "x-session-id", " \t "},
		{"too-long", "x-session-id", strings.Repeat("a", maxNativeSessionValueLength+1)},
		{"control", "x-session-id", "session\x7fvalue"},
		{"unknown", "x-promptgate-session-id", "custom"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header[test.header] = []string{test.value}
			called := false
			SessionContextMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				called = true
				if _, ok := NativeSessionFromContext(r.Context()); ok {
					t.Fatal("invalid or unknown value must not create session context")
				}
			})).ServeHTTP(httptest.NewRecorder(), req)
			if !called {
				t.Fatal("next handler was not called")
			}
		})
	}
}

func TestSessionContextMiddlewareExtractsW3CContextWithoutUsingItAsSession(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("baggage", "session.id=must-not-be-used")
	SessionContextMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		spanContext := traceSpanContext(r.Context())
		if got := spanContext.TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Fatalf("unexpected trace ID %q", got)
		}
		if _, ok := NativeSessionFromContext(r.Context()); ok {
			t.Fatal("W3C context must not become native session metadata")
		}
	})).ServeHTTP(httptest.NewRecorder(), req)
}

func TestSessionContextMiddlewareIgnoresInvalidW3CContext(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("traceparent", "invalid")
	called := false
	SessionContextMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if trace.SpanContextFromContext(r.Context()).IsValid() {
			t.Fatal("invalid traceparent must not create a valid span context")
		}
	})).ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Fatal("invalid W3C context must not block the request")
	}
}

func captureNativeSession(t *testing.T, req *http.Request) NativeSession {
	t.Helper()
	var got NativeSession
	SessionContextMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var ok bool
		got, ok = NativeSessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected native session context")
		}
	})).ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func traceSpanContext(ctx context.Context) trace.SpanContext {
	return trace.SpanContextFromContext(ctx)
}
