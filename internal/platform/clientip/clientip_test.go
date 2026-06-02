package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestResolveUsesRemoteAddrByDefault verifies forwarding headers are ignored unless trusted.
func TestResolveUsesRemoteAddrByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	req.Header.Set("X-Real-IP", "203.0.113.9")

	if got := Resolve(req, false); got != "192.0.2.10" {
		t.Fatalf("expected remote addr, got %q", got)
	}
}

// TestResolveUsesTrustedForwardedForFirstAddress verifies trusted X-Forwarded-For handling.
func TestResolveUsesTrustedForwardedForFirstAddress(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 203.0.113.9")

	if got := Resolve(req, true); got != "198.51.100.7" {
		t.Fatalf("expected first forwarded IP, got %q", got)
	}
}

// TestResolveUsesTrustedRealIPFallback verifies trusted X-Real-IP fallback handling.
func TestResolveUsesTrustedRealIPFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Real-IP", "203.0.113.9")

	if got := Resolve(req, true); got != "203.0.113.9" {
		t.Fatalf("expected real IP, got %q", got)
	}
}

// TestMiddlewareStoresResolvedClientIP verifies downstream handlers can read the resolved IP.
func TestMiddlewareStoresResolvedClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")

	var got string
	handler := Middleware(true)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = FromContext(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got != "198.51.100.7" {
		t.Fatalf("expected context client IP, got %q", got)
	}
}
