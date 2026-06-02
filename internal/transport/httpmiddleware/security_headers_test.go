package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSecurityHeadersAddsBaselineBrowserHardening verifies security headers adds baseline browser hardening.
func TestSecurityHeadersAddsBaselineBrowserHardening(t *testing.T) {
	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	expected := map[string]string{
		"Content-Security-Policy": "frame-ancestors 'none'; base-uri 'self'; object-src 'none'",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
		"Referrer-Policy":         "no-referrer",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
	}
	for header, value := range expected {
		if got := rec.Header().Get(header); got != value {
			t.Fatalf("expected %s %q, got %q", header, value, got)
		}
	}
}
