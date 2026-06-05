package clientip

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func mustPrefix(t *testing.T, value string) netip.Prefix {
	t.Helper()
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", value, err)
	}
	return prefix
}

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

// TestResolveWithOptionsUsesForwardedForFromTrustedProxy verifies CIDR-gated forwarded header trust.
func TestResolveWithOptionsUsesForwardedForFromTrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")

	got := ResolveWithOptions(req, Options{
		TrustedProxies: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
	})
	if got != "198.51.100.7" {
		t.Fatalf("expected forwarded IP from trusted proxy, got %q", got)
	}
}

// TestResolveWithOptionsIgnoresForwardedForFromUntrustedPeer verifies spoofed headers are ignored.
func TestResolveWithOptionsIgnoresForwardedForFromUntrustedPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")

	got := ResolveWithOptions(req, Options{
		TrustedProxies: []netip.Prefix{mustPrefix(t, "10.2.0.0/16")},
	})
	if got != "10.1.2.3" {
		t.Fatalf("expected untrusted peer remote addr, got %q", got)
	}
}

// TestResolveWithOptionsReadsForwardedForRightToLeft verifies the closest untrusted hop wins.
func TestResolveWithOptionsReadsForwardedForRightToLeft(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.66, 198.51.100.7, 10.9.8.7")

	got := ResolveWithOptions(req, Options{
		TrustedProxies: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
	})
	if got != "198.51.100.7" {
		t.Fatalf("expected closest untrusted forwarded IP, got %q", got)
	}
}

// TestResolveWithOptionsUsesTrustedRealIPFallback verifies X-Real-IP fallback with trusted peers.
func TestResolveWithOptionsUsesTrustedRealIPFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Real-IP", "198.51.100.7")

	got := ResolveWithOptions(req, Options{
		TrustedProxies: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
	})
	if got != "198.51.100.7" {
		t.Fatalf("expected real IP fallback, got %q", got)
	}
}

// TestResolveWithOptionsFallsBackToRemoteAddrForInvalidForwardedHeaders verifies bad headers are ignored.
func TestResolveWithOptionsFallsBackToRemoteAddrForInvalidForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "not-an-ip")
	req.Header.Set("X-Real-IP", "also-not-an-ip")

	got := ResolveWithOptions(req, Options{
		TrustedProxies: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
	})
	if got != "10.1.2.3" {
		t.Fatalf("expected remote addr fallback, got %q", got)
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

// TestMiddlewareWithOptionsStoresResolvedClientIP verifies option-based resolution reaches context.
func TestMiddlewareWithOptionsStoresResolvedClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")

	var got string
	handler := MiddlewareWithOptions(Options{
		TrustedProxies: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
	})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = FromContext(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got != "198.51.100.7" {
		t.Fatalf("expected context client IP, got %q", got)
	}
}
