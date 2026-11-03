package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserMutationBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, origin, referer, fetch, content string
		size                                  int
		status                                int
	}{
		{name: "trusted", origin: "https://app.test", content: "application/json", size: 2, status: 200},
		{name: "CLI", content: "application/json", size: 2, status: 200},
		{name: "sibling origin", origin: "https://evil.app.test", content: "text/plain", size: 2, status: 403},
		{name: "opaque origin", origin: "null", content: "application/json", size: 2, status: 403},
		{name: "bad referer", referer: "https://evil.test/form", content: "application/json", size: 2, status: 403},
		{name: "cross site", fetch: "cross-site", content: "application/json", size: 2, status: 403},
		{name: "simple form", content: "text/plain", size: 2, status: 415},
		{name: "too large", content: "application/json", size: int(MaxAPIRequestBytes) + 1, status: 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := BrowserRequests([]string{"https://app.test"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if _, err := io.ReadAll(r.Body); err != nil {
					w.WriteHeader(413)
				}
			}))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(strings.Repeat("x", tc.size)))
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Referer", tc.referer)
			req.Header.Set("Sec-Fetch-Site", tc.fetch)
			req.Header.Set("Content-Type", tc.content)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d", rec.Code, tc.status)
			}
			if tc.status != 200 && tc.status != 413 && called {
				t.Fatal("rejected mutation reached the handler")
			}
		})
	}
}
