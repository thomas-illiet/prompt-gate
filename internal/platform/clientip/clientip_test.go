package clientip

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"promptgate/backend/internal/platform/requestprivacy"
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

// TestResolveWithOptionsGlobalTrustOverridesTrustedProxyCIDRs verifies the legacy switch keeps global trust semantics.
func TestResolveWithOptionsGlobalTrustOverridesTrustedProxyCIDRs(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 203.0.113.9")

	got := ResolveWithOptions(req, Options{
		TrustForwardHeaders: true,
		TrustedProxies:      []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
	})
	if got != "198.51.100.7" {
		t.Fatalf("expected globally trusted forwarded IP, got %q", got)
	}
}

// TestResolveGlobalTrustKeepsFirstHeaderSemantics verifies the legacy global
// switch is unaffected by the stricter trusted-proxy chain handling.
func TestResolveGlobalTrustKeepsFirstHeaderSemantics(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Add("X-Forwarded-For", "198.51.100.7")
	req.Header.Add("X-Forwarded-For", "203.0.113.9")

	if got := Resolve(req, true); got != "198.51.100.7" {
		t.Fatalf("expected first forwarded IP from legacy global trust, got %q", got)
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

// TestResolveWithOptionsReadsRepeatedForwardedForRightToLeft verifies that
// HAProxy's appended header wins over values supplied by an untrusted client.
func TestResolveWithOptionsReadsRepeatedForwardedForRightToLeft(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Add("X-Forwarded-For", "203.0.113.66") // client-supplied spoof
	req.Header.Add("X-Forwarded-For", "198.51.100.7") // client recorded by F5
	req.Header.Add("X-Forwarded-For", "192.0.2.9")    // F5 recorded by HAProxy

	got := ResolveWithOptions(req, Options{
		TrustedProxies: []netip.Prefix{
			mustPrefix(t, "10.0.0.0/8"),
			mustPrefix(t, "192.0.2.0/24"),
		},
	})
	if got != "198.51.100.7" {
		t.Fatalf("expected F5-recorded client IP, got %q", got)
	}
}

// TestResolveWithOptionsReadsRepeatedAndCommaSeparatedForwardedFor verifies
// every header occurrence participates in the same ordered proxy chain.
func TestResolveWithOptionsReadsRepeatedAndCommaSeparatedForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Add("X-Forwarded-For", "203.0.113.66, 203.0.113.67")
	req.Header.Add("X-Forwarded-For", "198.51.100.7, 192.0.2.9")

	got := ResolveWithOptions(req, Options{
		TrustedProxies: []netip.Prefix{
			mustPrefix(t, "10.0.0.0/8"),
			mustPrefix(t, "192.0.2.0/24"),
		},
	})
	if got != "198.51.100.7" {
		t.Fatalf("expected closest untrusted client IP, got %q", got)
	}
}

// TestResolveWithOptionsRejectsMalformedForwardedForChain verifies invalid hops stop trust traversal.
func TestResolveWithOptionsRejectsMalformedForwardedForChain(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.66, not-an-ip, 10.9.8.7")
	req.Header.Set("X-Real-IP", "203.0.113.9")

	got := ResolveWithOptions(req, Options{
		TrustedProxies: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
	})
	if got != "10.1.2.3" {
		t.Fatalf("expected malformed chain to fall back to remote addr, got %q", got)
	}
}

// TestResolveWithOptionsRejectsMalformedRepeatedForwardedForChain verifies a
// malformed hop between the client and trusted edge fails closed.
func TestResolveWithOptionsRejectsMalformedRepeatedForwardedForChain(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Add("X-Forwarded-For", "198.51.100.7")
	req.Header.Add("X-Forwarded-For", "not-an-ip")
	req.Header.Add("X-Forwarded-For", "192.0.2.9")
	req.Header.Set("X-Real-IP", "203.0.113.9")

	got := ResolveWithOptions(req, Options{
		TrustedProxies: []netip.Prefix{
			mustPrefix(t, "10.0.0.0/8"),
			mustPrefix(t, "192.0.2.0/24"),
		},
	})
	if got != "10.1.2.3" {
		t.Fatalf("expected malformed repeated chain to fall back to remote addr, got %q", got)
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

// TestMiddlewareClientIPDebugLogsOnlyResolutionMetadata verifies diagnostics omit raw headers and request content.
func TestMiddlewareClientIPDebugLogsOnlyResolutionMetadata(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat?token=query-secret", strings.NewReader("body-secret"))
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 10.9.8.7")
	req.Header.Set("Authorization", "Bearer header-secret")

	handler := MiddlewareWithOptions(Options{
		TrustedProxies: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
		Debug:          true,
		Logger:         logger,
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	logged := output.String()
	for _, expected := range []string{`"peer_ip":"10.1.2.3"`, `"client_ip":"198.51.100.7"`, `"source":"x-forwarded-for"`, `"peer_trusted":true`} {
		if !strings.Contains(logged, expected) {
			t.Fatalf("expected diagnostic field %s in %s", expected, logged)
		}
	}
	for _, secret := range []string{"10.9.8.7", "query-secret", "body-secret", "header-secret", "Authorization", "X-Forwarded-For"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("diagnostic log exposed %q: %s", secret, logged)
		}
	}
}

func TestMiddlewareClientIPDebugSuppressesMetadataOnlyRequests(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/messages", nil)
	req.RemoteAddr = "198.51.100.77:4242"
	req = req.WithContext(requestprivacy.MarkMetadataOnly(req.Context()))

	handler := MiddlewareWithOptions(Options{Debug: true, Logger: logger})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			if FromContext(r.Context()) != "198.51.100.77" {
				t.Fatal("client IP must remain available to authorization")
			}
		}),
	)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if output.Len() != 0 {
		t.Fatalf("metadata-only request logged client IP: %s", output.String())
	}
}

// TestMiddlewareClientIPDebugDisabledByDefault verifies resolution emits no diagnostic unless opted in.
func TestMiddlewareClientIPDebugDisabledByDefault(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")

	handler := MiddlewareWithOptions(Options{
		TrustedProxies: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
		Logger:         logger,
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if output.Len() != 0 {
		t.Fatalf("expected no diagnostic log, got %s", output.String())
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
