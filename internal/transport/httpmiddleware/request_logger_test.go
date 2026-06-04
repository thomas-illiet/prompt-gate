package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"promptgate/backend/internal/platform/clientip"
)

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
