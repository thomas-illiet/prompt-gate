package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStaticAssetsHandlerServesFilesAndSPAFallback verifies static assets handler serves files and SPA fallback.
func TestStaticAssetsHandlerServesFilesAndSPAFallback(t *testing.T) {
	root := t.TempDir()
	writeStaticTestFile(t, root, "index.html", "index shell")
	writeStaticTestFile(t, root, "200.html", "spa shell")
	writeStaticTestFile(t, root, "_nuxt/app.js", "console.log('ok')")
	writeStaticTestFile(t, root, "favicon.ico", "ico")

	handler := newStaticAssetsHandler(root)

	tests := []struct {
		name       string
		path       string
		statusCode int
		body       string
	}{
		{
			name:       "root serves index",
			path:       "/",
			statusCode: http.StatusOK,
			body:       "index shell",
		},
		{
			name:       "frontend route serves spa fallback",
			path:       "/dashboard",
			statusCode: http.StatusOK,
			body:       "spa shell",
		},
		{
			name:       "existing asset is served",
			path:       "/_nuxt/app.js",
			statusCode: http.StatusOK,
			body:       "console.log('ok')",
		},
		{
			name:       "existing root asset is served",
			path:       "/favicon.ico",
			statusCode: http.StatusOK,
			body:       "ico",
		},
		{
			name:       "missing asset is not rewritten to spa",
			path:       "/_nuxt/missing.js",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "missing file-like route is not rewritten to spa",
			path:       "/images/logo.png",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "unknown api path remains backend 404",
			path:       "/api/v1/unknown",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "unknown auth path remains backend 404",
			path:       "/auth/unknown",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "health path remains backend 404",
			path:       "/health",
			statusCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			if recorder.Code != test.statusCode {
				t.Fatalf("expected status %d, got %d", test.statusCode, recorder.Code)
			}
			if test.body != "" && !strings.Contains(recorder.Body.String(), test.body) {
				t.Fatalf("expected response body to contain %q, got %q", test.body, recorder.Body.String())
			}
		})
	}
}

// TestStaticAssetsHandlerFallsBackToIndexWhen200IsMissing verifies static assets handler falls back to index when 200 is missing.
func TestStaticAssetsHandlerFallsBackToIndexWhen200IsMissing(t *testing.T) {
	root := t.TempDir()
	writeStaticTestFile(t, root, "index.html", "index shell")

	req := httptest.NewRequest(http.MethodGet, "/tokens", nil)
	recorder := httptest.NewRecorder()

	newStaticAssetsHandler(root).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "index shell") {
		t.Fatalf("expected index fallback body, got %q", recorder.Body.String())
	}
}

// writeStaticTestFile writes static test file.
func writeStaticTestFile(t *testing.T, root string, name string, content string) {
	t.Helper()

	filePath := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("create static fixture dir: %v", err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write static fixture %s: %v", name, err)
	}
}
