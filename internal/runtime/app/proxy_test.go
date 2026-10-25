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

func TestProxyBasePathMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		wantPath    string
		wantStatus  int
		wantCalled  bool
	}{
		{name: "prefixed provider route", requestPath: "/remote/openai-main/v1/models", wantPath: "/openai-main/v1/models", wantStatus: http.StatusNoContent, wantCalled: true},
		{name: "prefixed health", requestPath: "/remote/health", wantPath: "/health", wantStatus: http.StatusNoContent, wantCalled: true},
		{name: "direct health", requestPath: "/health", wantPath: "/health", wantStatus: http.StatusNoContent, wantCalled: true},
		{name: "base root", requestPath: "/remote", wantPath: "/", wantStatus: http.StatusNoContent, wantCalled: true},
		{name: "segment lookalike", requestPath: "/remoteevil/openai-main/v1/models", wantStatus: http.StatusNotFound},
		{name: "missing prefix", requestPath: "/openai-main/v1/models", wantStatus: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := proxyBasePathMiddleware("/remote", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.URL.Path != test.wantPath {
					t.Fatalf("normalized path = %q, want %q", r.URL.Path, test.wantPath)
				}
				if r.URL.RawPath != "" {
					t.Fatalf("expected canonical empty RawPath, got %q", r.URL.RawPath)
				}
				if r.URL.RawQuery != "tenant=one" {
					t.Fatalf("query = %q, want tenant=one", r.URL.RawQuery)
				}
				if r.RequestURI != test.wantPath+"?tenant=one" {
					t.Fatalf("request URI = %q, want %q", r.RequestURI, test.wantPath+"?tenant=one")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodGet, test.requestPath+"?tenant=one", nil)
			if test.requestPath != "/health" {
				req.URL.RawPath = test.requestPath
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if called != test.wantCalled {
				t.Fatalf("downstream called = %t, want %t", called, test.wantCalled)
			}
		})
	}
}

func TestProxyBasePathMiddlewareRejectsNonCanonicalPaths(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		rawPath string
	}{
		{name: "double slash", target: "/remote//openai-main/v1/models?tenant=one"},
		{name: "dot segment", target: "/remote/./openai-main/v1/models?tenant=one"},
		{name: "dot dot segment", target: "/remote/openai-main/../anthropic-main/v1/models?tenant=one"},
		{name: "encoded slash in prefix boundary", target: "/remote%2Fopenai-main/v1/models?tenant=one"},
		{name: "encoded slash in suffix", target: "/remote/openai-main%2fv1/models?tenant=one"},
		{name: "encoded dot segment", target: "/remote/%2e/openai-main/v1/models?tenant=one"},
		{name: "encoded dot dot segment", target: "/remote/%2E%2E/openai-main/v1/models?tenant=one"},
		{name: "encoded backslash", target: "/remote/openai-main%5Cv1/models?tenant=one"},
		{name: "raw path does not match path", target: "/remote/openai-main/v1/models?tenant=one", rawPath: "/remote/%61nother/v1/models"},
		{name: "malformed raw path", target: "/remote/openai-main/v1/models?tenant=one", rawPath: "/remote/%zz/v1/models"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := proxyBasePathMiddleware("/remote", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodGet, test.target, nil)
			if test.rawPath != "" {
				req.URL.RawPath = test.rawPath
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
			}
			if called {
				t.Fatal("non-canonical request reached downstream handler")
			}
			if location := recorder.Header().Get("Location"); location != "" {
				t.Fatalf("unexpected redirect location %q", location)
			}
		})
	}
}

func TestProxyBasePathMiddlewareAllowsUnambiguousRawPath(t *testing.T) {
	handler := proxyBasePathMiddleware("/remote", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/provider/v1/models" {
			t.Fatalf("normalized path = %q, want /provider/v1/models", r.URL.Path)
		}
		if r.URL.RawPath != "" {
			t.Fatalf("expected canonical empty RawPath, got %q", r.URL.RawPath)
		}
		if r.URL.RawQuery != "tenant=one" {
			t.Fatalf("query = %q, want tenant=one", r.URL.RawQuery)
		}
		if r.RequestURI != "/provider/v1/models?tenant=one" {
			t.Fatalf("request URI = %q, want /provider/v1/models?tenant=one", r.RequestURI)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/remote/%70rovider/v1/models?tenant=one", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestProxyBasePathMiddlewareEmptyLeavesRequestUntouched(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://proxy.example.test/openai-main/v1/models?tenant=one", nil)
	req.URL.RawPath = "/openai-main/v1/models"
	handler := proxyBasePathMiddleware("", http.HandlerFunc(func(w http.ResponseWriter, got *http.Request) {
		if got != req || got.URL.Path != "/openai-main/v1/models" || got.URL.RawPath != "/openai-main/v1/models" {
			t.Fatalf("empty base path changed request: %#v", got.URL)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
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
			ProxyBasePath:                 "/remote",
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
	prefixedHealthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(prefixedHealthRecorder, httptest.NewRequest(http.MethodGet, "/remote/health", nil))
	if prefixedHealthRecorder.Code != http.StatusServiceUnavailable || prefixedHealthRecorder.Body.String() != `{"status":"degraded","dependency":"opa"}` {
		t.Fatalf("unexpected prefixed health response: %d %q", prefixedHealthRecorder.Code, prefixedHealthRecorder.Body.String())
	}

	directRequest := httptest.NewRequest(http.MethodPost, "/openai/v1/responses", strings.NewReader(`{"model":"gpt-test"}`))
	directRecorder := httptest.NewRecorder()
	handler.ServeHTTP(directRecorder, directRequest)
	if directRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected direct provider route to stop at 404, got %d: %s", directRecorder.Code, directRecorder.Body.String())
	}

	proxyRequest := httptest.NewRequest(http.MethodPost, "/remote/openai/v1/responses", strings.NewReader(`{"model":"gpt-test"}`))
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
	if !strings.Contains(debugOutput.String(), `"path":"/openai/v1/responses"`) || strings.Contains(debugOutput.String(), `"path":"/remote/openai/v1/responses"`) {
		t.Fatalf("debug middleware did not observe canonical path: %s", debugOutput.String())
	}

	remoteProviderRequest := httptest.NewRequest(http.MethodGet, "/remote/remote/v1/models", nil)
	remoteProviderRecorder := httptest.NewRecorder()
	handler.ServeHTTP(remoteProviderRecorder, remoteProviderRequest)
	if remoteProviderRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected provider named remote to reach auth, got %d: %s", remoteProviderRecorder.Code, remoteProviderRecorder.Body.String())
	}
	if !strings.Contains(debugOutput.String(), `"path":"/remote/v1/models"`) {
		t.Fatalf("provider named remote was not normalized exactly once: %s", debugOutput.String())
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
