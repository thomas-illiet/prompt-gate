package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCORSPermitsConfiguredOriginWithCredentials verifies CORS permits configured origin with credentials.
func TestCORSPermitsConfiguredOriginWithCredentials(t *testing.T) {
	handler := CORS([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPut)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Idempotency-Key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("unexpected allowed origin: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("expected credentials header, got %q", rec.Header().Get("Access-Control-Allow-Credentials"))
	}
	allowedHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	if allowedHeaders != "Authorization, Content-Type, Accept, Idempotency-Key" {
		t.Fatalf("unexpected allowed headers: %q", allowedHeaders)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
		t.Fatalf("unexpected allowed methods: %q", rec.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestCORSPreflightDoesNotExposeCredentialHeaders(t *testing.T) {
	handler := CORS([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Idempotency-Key, Cookie, X-Admin-API-Key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	allowedHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(strings.ToLower(allowedHeaders), strings.ToLower("Idempotency-Key")) {
		t.Fatalf("idempotency header is missing from CORS response: %q", allowedHeaders)
	}
	for _, forbidden := range []string{"Cookie", AdminAPIKeyHeader} {
		if strings.Contains(strings.ToLower(allowedHeaders), strings.ToLower(forbidden)) {
			t.Fatalf("forbidden header %q was exposed by CORS: %q", forbidden, allowedHeaders)
		}
	}
}
