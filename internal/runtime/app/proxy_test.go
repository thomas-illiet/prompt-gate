package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"promptgate/backend/internal/domain/policy"
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
	proxyHealth(client).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

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
	proxyHealth(client).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusServiceUnavailable || recorder.Body.String() != `{"status":"degraded","dependency":"opa"}` {
		t.Fatalf("unexpected degraded health: %d %q", recorder.Code, recorder.Body.String())
	}

	status.Store(http.StatusOK)
	recorder = httptest.NewRecorder()
	proxyHealth(client).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected recovered health, got %d", recorder.Code)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
