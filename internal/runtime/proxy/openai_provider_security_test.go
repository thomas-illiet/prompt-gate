package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOpenAIPassthroughAllowsOnlyExactModelList(t *testing.T) {
	var upstreamCalls atomic.Int32
	var upstreamHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		upstreamHeaders = r.Header.Clone()
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Errorf("provider authorization: got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	t.Cleanup(upstream.Close)

	bridge := newEmbeddingTestBridge(t, newOpenAIProvider("openai", upstream.URL+"/v1", "provider-key"), &embeddingTestRecorder{})
	t.Cleanup(func() { _ = bridge.Shutdown(context.Background()) })
	manager := &Manager{}
	manager.installBridge(bridge)
	handler := MethodGuardMiddleware(manager)

	for _, test := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodDelete, path: "/openai/v1/models", status: http.StatusMethodNotAllowed},
		{method: http.MethodDelete, path: "/openai/v1/models/fine-tuned-model", status: http.StatusNotFound},
		{method: http.MethodDelete, path: "/openai/v1/conversations", status: http.StatusNotFound},
		{method: http.MethodDelete, path: "/openai/v1/conversations/conv_123", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/openai/v1/conversations/conv_123/items", status: http.StatusNotFound},
		{method: http.MethodDelete, path: "/openai/v1/responses", status: http.StatusMethodNotAllowed},
		{method: http.MethodDelete, path: "/openai/v1/responses/resp_123", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/openai/v1/responses/resp_123", status: http.StatusNotFound},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			if response.Code != test.status {
				t.Fatalf("status: got %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			if upstreamCalls.Load() != 0 {
				t.Fatalf("forbidden route reached upstream %d time(s)", upstreamCalls.Load())
			}
		})
	}

	request := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cookie", "promptgate_session=session-secret")
	request.Header.Set("X-Admin-API-Key", "admin-secret")
	request.Header.Set("X-Custom-Secret", "custom-secret")
	request.Header.Set("X-Forwarded-For", "198.51.100.77")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || upstreamCalls.Load() != 1 {
		t.Fatalf("safe model list: status=%d calls=%d body=%s", response.Code, upstreamCalls.Load(), response.Body.String())
	}
	for _, name := range []string{"Cookie", "X-Admin-API-Key", "X-Custom-Secret"} {
		if got := upstreamHeaders.Get(name); got != "" {
			t.Errorf("secret header %s reached upstream: %q", name, got)
		}
	}
	if got := upstreamHeaders.Get("X-Forwarded-For"); strings.Contains(got, "198.51.100.77") {
		t.Errorf("untrusted forwarded header reached upstream: %q", got)
	}
}
