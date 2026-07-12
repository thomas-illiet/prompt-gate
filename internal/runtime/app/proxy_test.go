package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
	recorder := httptest.NewRecorder()
	proxyHealth(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content type: %q", recorder.Header().Get("Content-Type"))
	}
	if recorder.Body.String() != `{"status":"ok"}` {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}
}
