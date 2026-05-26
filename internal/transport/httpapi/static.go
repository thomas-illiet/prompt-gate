package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type staticAssetsHandler struct {
	root string
}

func newStaticAssetsHandler(root string) http.Handler {
	return staticAssetsHandler{root: root}
}

func (h staticAssetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	requestPath := cleanRequestPath(r.URL.Path)
	if isBackendPath(requestPath) {
		http.NotFound(w, r)
		return
	}

	if requestPath == "/" {
		if h.serveNamedFile(w, r, "index.html") {
			return
		}
		http.NotFound(w, r)
		return
	}

	if h.serveRequestFile(w, r, requestPath) {
		return
	}

	if isAssetPath(requestPath) {
		http.NotFound(w, r)
		return
	}

	if h.serveNamedFile(w, r, "200.html") || h.serveNamedFile(w, r, "index.html") {
		return
	}

	http.NotFound(w, r)
}

func cleanRequestPath(value string) string {
	if value == "" {
		return "/"
	}

	return path.Clean("/" + strings.TrimPrefix(value, "/"))
}

func isBackendPath(requestPath string) bool {
	return requestPath == "/health" ||
		requestPath == "/api" ||
		strings.HasPrefix(requestPath, "/api/") ||
		requestPath == "/auth" ||
		strings.HasPrefix(requestPath, "/auth/")
}

func isAssetPath(requestPath string) bool {
	return strings.HasPrefix(requestPath, "/_nuxt/") || path.Ext(requestPath) != ""
}

func (h staticAssetsHandler) serveRequestFile(w http.ResponseWriter, r *http.Request, requestPath string) bool {
	relativePath := strings.TrimPrefix(requestPath, "/")
	if relativePath == "" {
		return false
	}

	return h.serveNamedFile(w, r, relativePath)
}

func (h staticAssetsHandler) serveNamedFile(w http.ResponseWriter, r *http.Request, name string) bool {
	filePath, ok := h.resolveFile(name)
	if !ok {
		return false
	}

	http.ServeFile(w, r, filePath)
	return true
}

func (h staticAssetsHandler) resolveFile(name string) (string, bool) {
	root, err := filepath.Abs(h.root)
	if err != nil {
		return "", false
	}

	candidate, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return "", false
	}

	if candidate != root && !strings.HasPrefix(candidate, root+string(os.PathSeparator)) {
		return "", false
	}

	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return "", false
	}

	return candidate, true
}
