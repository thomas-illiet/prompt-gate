package firewall

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/clientip"
)

func mustFirewallPrefix(t *testing.T, value string) netip.Prefix {
	t.Helper()
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", value, err)
	}
	return prefix
}

// TestMiddlewareAllowsByDefaultAndDeniesMatchingRule verifies firewall default allow and deny matches.
func TestMiddlewareAllowsByDefaultAndDeniesMatchingRule(t *testing.T) {
	snapshot := NewSnapshotStore(nil)
	nextCalled := false
	handler := Middleware(snapshot, false, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.10:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || !nextCalled {
		t.Fatalf("expected default allow, code=%d next=%v", rec.Code, nextCalled)
	}

	nextCalled = false
	snapshot.Set([]FirewallRule{{ID: "rule-1", Address: "192.168.1.10", Priority: 1, Action: ActionDeny, Enabled: true}})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || nextCalled {
		t.Fatalf("expected deny, code=%d next=%v", rec.Code, nextCalled)
	}
}

// TestMiddlewareWithOptionsUsesResolvedTrustedForwardedIP verifies firewall and context share resolution.
func TestMiddlewareWithOptionsUsesResolvedTrustedForwardedIP(t *testing.T) {
	snapshot := NewSnapshotStore(nil)
	snapshot.Set([]FirewallRule{{ID: "rule-1", Address: "198.51.100.7", Priority: 1, Action: ActionDeny, Enabled: true}})
	options := clientip.Options{
		TrustedProxies: []netip.Prefix{mustFirewallPrefix(t, "10.0.0.0/8")},
	}
	nextCalled := false
	handler := clientip.MiddlewareWithOptions(options)(
		MiddlewareWithOptions(snapshot, options, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || nextCalled {
		t.Fatalf("expected trusted forwarded IP deny, code=%d next=%v", rec.Code, nextCalled)
	}
}

// TestMiddlewareWithOptionsIgnoresForwardedIPFromUntrustedPeer verifies spoofed headers do not drive firewall rules.
func TestMiddlewareWithOptionsIgnoresForwardedIPFromUntrustedPeer(t *testing.T) {
	snapshot := NewSnapshotStore(nil)
	snapshot.Set([]FirewallRule{{ID: "rule-1", Address: "198.51.100.7", Priority: 1, Action: ActionDeny, Enabled: true}})
	options := clientip.Options{
		TrustedProxies: []netip.Prefix{mustFirewallPrefix(t, "10.2.0.0/16")},
	}
	nextCalled := false
	handler := clientip.MiddlewareWithOptions(options)(
		MiddlewareWithOptions(snapshot, options, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent || !nextCalled {
		t.Fatalf("expected untrusted forwarded IP to be ignored, code=%d next=%v", rec.Code, nextCalled)
	}
}

// TestMiddlewareNormalizesIPv6Loopback verifies local proxy calls from ::1 are evaluated as IPv4 loopback.
func TestMiddlewareNormalizesIPv6Loopback(t *testing.T) {
	snapshot := NewSnapshotStore(nil)
	snapshot.Set([]FirewallRule{{ID: "rule-1", Address: "127.0.0.1", Priority: 1, Action: ActionDeny, Enabled: true}})
	nextCalled := false
	handler := Middleware(snapshot, false, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::1]:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || nextCalled {
		t.Fatalf("expected IPv6 loopback to match IPv4 loopback deny, code=%d next=%v", rec.Code, nextCalled)
	}
}

// TestParseClientAddrRejectsNonLoopbackIPv6 keeps the firewall scoped to IPv4 rules.
func TestParseClientAddrRejectsNonLoopbackIPv6(t *testing.T) {
	if _, err := parseClientAddr("2001:db8::1"); err != ErrInvalidAddress {
		t.Fatalf("expected invalid IPv4 address, got %v", err)
	}
}

// TestMiddlewareUsesServiceAccountOverride verifies middleware uses service account override.
func TestMiddlewareUsesServiceAccountOverride(t *testing.T) {
	serviceAccountID := "service-account-id"
	snapshot := NewSnapshotStore(nil)
	snapshot.SetSnapshot(Snapshot{
		Global: []FirewallRule{{ID: "global-deny", Address: "10.0.0.10", Priority: 1, Action: ActionDeny, Enabled: true}},
		ServiceAccounts: map[string][]FirewallRule{
			serviceAccountID: {
				{ID: "scoped-allow", Type: RuleTypeServiceAccount, ReferentielID: &serviceAccountID, Address: "10.0.0.10", Priority: 1, Action: ActionAllow, Enabled: true},
			},
		},
	})
	nextCalled := false
	handler := Middleware(snapshot, false, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.10:1234"
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:                      serviceAccountID,
		Type:                    auth.UserTypeService,
		FirewallOverrideEnabled: true,
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent || !nextCalled {
		t.Fatalf("expected scoped allow to pass, code=%d next=%v", rec.Code, nextCalled)
	}
}

// TestMiddlewareServiceAccountOverrideDefaultsToDeny verifies middleware service account override defaults to deny.
func TestMiddlewareServiceAccountOverrideDefaultsToDeny(t *testing.T) {
	serviceAccountID := "service-account-id"
	snapshot := NewSnapshotStore(nil)
	snapshot.SetSnapshot(Snapshot{
		Global: []FirewallRule{{ID: "global-allow", Address: "10.0.0.10", Priority: 1, Action: ActionAllow, Enabled: true}},
		ServiceAccounts: map[string][]FirewallRule{
			serviceAccountID: {},
		},
	})
	nextCalled := false
	handler := Middleware(snapshot, false, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.10:1234"
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:                      serviceAccountID,
		Type:                    auth.UserTypeService,
		FirewallOverrideEnabled: true,
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || nextCalled {
		t.Fatalf("expected scoped default deny, code=%d next=%v", rec.Code, nextCalled)
	}
}
