package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCORSPermitsConfiguredOriginWithCredentials verifies CORS permits configured origin with credentials.
func TestCORSPermitsConfiguredOriginWithCredentials(t *testing.T) {
	handler := CORS([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
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
	if rec.Header().Get("Access-Control-Allow-Headers") != "Authorization, Content-Type, Accept" {
		t.Fatalf("unexpected allowed headers: %q", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}
