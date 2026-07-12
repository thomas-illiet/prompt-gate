package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/platform/clientip"
)

// TestRequestLoggerOmitsQuery verifies credentials and OAuth parameters never reach request logs.
func TestRequestLoggerOmitsQuery(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(
		http.MethodGet,
		"/oauth/callback?code=secret-code&state=secret-state&token=secret-token&api_key=secret-key",
		nil,
	)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	if err := json.NewDecoder(&output).Decode(&entry); err != nil {
		t.Fatalf("decode request log: %v", err)
	}
	if got := entry["method"]; got != http.MethodGet {
		t.Fatalf("expected method %q, got %v", http.MethodGet, got)
	}
	if got := entry["path"]; got != "/oauth/callback" {
		t.Fatalf("expected path %q, got %v", "/oauth/callback", got)
	}
	if got := entry["status"]; got != float64(http.StatusNoContent) {
		t.Fatalf("expected status %d, got %v", http.StatusNoContent, got)
	}
	if _, exists := entry["query"]; exists {
		t.Fatal("request log must not contain a query attribute")
	}
	for _, secret := range []string{"secret-code", "secret-state", "secret-token", "secret-key"} {
		if strings.Contains(output.String(), secret) {
			t.Errorf("request log contains sensitive query value %q", secret)
		}
	}
}

// TestRequestRemoteAddrPrefersResolvedContextIP verifies trusted resolution happens outside logging.
func TestRequestRemoteAddrPrefersResolvedContextIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:4312"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")
	req = req.WithContext(clientip.ContextWithClientIP(req.Context(), "192.0.2.44"))

	if got := requestRemoteAddr(req); got != "192.0.2.44" {
		t.Fatalf("expected resolved context IP, got %q", got)
	}
}

// TestRequestRemoteAddrIgnoresForwardHeadersWithoutResolvedContext verifies spoofed headers are ignored.
func TestRequestRemoteAddrIgnoresForwardHeadersWithoutResolvedContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:4312"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")
	req.Header.Set("X-Real-IP", "198.51.100.10")

	if got := requestRemoteAddr(req); got != "203.0.113.10" {
		t.Fatalf("expected remote address host, got %q", got)
	}
}
