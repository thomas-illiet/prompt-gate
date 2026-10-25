package app

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"promptgate/backend/internal/domain/policy"
	"promptgate/backend/internal/platform/config"
	httpmiddleware "promptgate/backend/internal/transport/httpmiddleware"
)

func TestRequestTimeoutAddsRequestDeadline(t *testing.T) {
	const timeout = 250 * time.Millisecond
	var remaining time.Duration
	handler := requestTimeout(timeout)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("expected request deadline")
		}
		remaining = time.Until(deadline)
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/provider/v1/chat", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if remaining <= 0 || remaining > timeout {
		t.Fatalf("expected deadline within %s, got %s", timeout, remaining)
	}
}

func TestProxyHealth(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusOK)
	client, err := policy.NewClient("http://opa:8181", "promptgate/proxy/decision", &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: int(status.Load()), Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header), Request: req}, nil
	})})
	if err != nil {
		t.Fatalf("new OPA client: %v", err)
	}
	recorder := httptest.NewRecorder()
	proxyHealth(true, client).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content type: %q", recorder.Header().Get("Content-Type"))
	}
	if recorder.Body.String() != `{"status":"ok"}` {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}

	status.Store(http.StatusServiceUnavailable)
	recorder = httptest.NewRecorder()
	proxyHealth(true, client).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusServiceUnavailable || recorder.Body.String() != `{"status":"degraded","dependency":"opa"}` {
		t.Fatalf("unexpected degraded health: %d %q", recorder.Code, recorder.Body.String())
	}

	status.Store(http.StatusOK)
	recorder = httptest.NewRecorder()
	proxyHealth(true, client).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected recovered health, got %d", recorder.Code)
	}
}

func TestProxyHealthSkipsOPAWhenDisabled(t *testing.T) {
	recorder := httptest.NewRecorder()
	proxyHealth(false, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"status":"ok"}` {
		t.Fatalf("unexpected disabled OPA health: %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestBuildHandlerComposesOptionalLayersAndKeepsHealthIndependent(t *testing.T) {
	var debugOutput bytes.Buffer
	runtime := &ProxyRuntime{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	cfg := config.ProxyConfig{
		ServerConfig: config.ServerConfig{
			CORSAllowedOrigins: []string{"https://frontend.example.test"},
		},
		ProxyRuntimeConfig: config.ProxyRuntimeConfig{
			ProxyMaxBufferedRequestBytes:  1 << 20,
			ProxyMaxBufferedResponseBytes: 1 << 20,
			ProxyUpstreamTimeout:          time.Second,
			OPAEnabled:                    true,
		},
		ModelLab: config.ModelLabConfig{
			MaxPromptBytes:  1024,
			MaxOutputTokens: 16,
		},
		OTel: config.OTelConfig{
			CaptureOutput:   true,
			CaptureThinking: true,
		},
	}
	handler := runtime.buildHandler(
		cfg,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		httpmiddleware.NewJSONLineWriter(&debugOutput),
	)

	healthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if healthRecorder.Code != http.StatusServiceUnavailable || healthRecorder.Body.String() != `{"status":"degraded","dependency":"opa"}` {
		t.Fatalf("unexpected health response: %d %q", healthRecorder.Code, healthRecorder.Body.String())
	}
	if got := healthRecorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("security header = %q", got)
	}

	proxyRequest := httptest.NewRequest(http.MethodPost, "/openai/v1/responses", strings.NewReader(`{"model":"gpt-test"}`))
	proxyRequest.Header.Set("Origin", "https://frontend.example.test")
	proxyRecorder := httptest.NewRecorder()
	handler.ServeHTTP(proxyRecorder, proxyRequest)
	if proxyRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated proxy request to stop at 401, got %d: %s", proxyRecorder.Code, proxyRecorder.Body.String())
	}
	if got := proxyRecorder.Header().Get("Access-Control-Allow-Origin"); got != "https://frontend.example.test" {
		t.Fatalf("CORS origin = %q", got)
	}
	if !strings.Contains(debugOutput.String(), `"event":"proxy_request_debug"`) {
		t.Fatalf("debug middleware did not observe proxy request: %s", debugOutput.String())
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
