package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cdrslog "cdr.dev/slog/v3"
	"github.com/alicebob/miniredis/v2"
	"promptgate/backend/internal/domain/policy"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/redisstore"
	proxyruntime "promptgate/backend/internal/runtime/proxy"
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
			IPValidationEnabled:           true,
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
		httpmiddleware.NewJSONLineWriter(&debugOutput),
	)

	for _, path := range []string{"/live", "/remote/live"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("liveness must not depend on external services: %d", rec.Code)
		}
	}
	healthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if healthRecorder.Code != http.StatusServiceUnavailable || !strings.Contains(healthRecorder.Body.String(), "dependencies unavailable") {
		t.Fatalf("unexpected health response: %d %q", healthRecorder.Code, healthRecorder.Body.String())
	}
	if got := healthRecorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("security header = %q", got)
	}
	prefixedHealthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(prefixedHealthRecorder, httptest.NewRequest(http.MethodGet, "/remote/health", nil))
	if prefixedHealthRecorder.Code != http.StatusServiceUnavailable || !strings.Contains(prefixedHealthRecorder.Body.String(), "dependencies unavailable") {
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

func TestHealthyProxyDoesNotProbeIPValidation(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	store, err := redisstore.NewRequired(ctx, "redis://"+server.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fixture := newModelLabProxyFixture(t)
	providers := provider.NewService(fixture.db, nil)
	if err := providers.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := providers.CreateProvider(ctx, provider.CreateProviderInput{Name: "test", Type: provider.ProviderTypeOpenAI, BaseURL: "http://127.0.0.1:1/v1", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	manager, err := proxyruntime.NewManager(ctx, proxyruntime.Options{
		Providers:    providers,
		BridgeLogger: cdrslog.Make(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown(ctx)
	runtime := &ProxyRuntime{redis: store, manager: manager, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	client, err := policy.NewClient("https://validation.example/ips/{ip}", "secret", &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Error("health reached external IP service")
		return nil, context.DeadlineExceeded
	})})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.ProxyConfig{ProxyRuntimeConfig: config.ProxyRuntimeConfig{IPValidationEnabled: true, ProxyBasePath: "/remote"}}
	handler := runtime.buildHandler(cfg, nil, nil, nil, policy.NewEvaluator(client, store, time.Minute, nil), nil, nil, nil, nil)
	for _, path := range []string{"/health", "/remote/health", "/live"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 {
			t.Fatalf("healthy proxy unavailable at %s: %d %s", path, rec.Code, rec.Body)
		}
	}
	_ = store.Close()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != 503 {
		t.Fatal("Redis failure must still degrade health")
	}
}
