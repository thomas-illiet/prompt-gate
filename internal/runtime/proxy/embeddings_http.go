package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var responseHeadersExcludedFromCopy = map[string]struct{}{
	"Connection":          {},
	"Content-Length":      {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

// embeddingModel extracts the requested model from an embeddings JSON body.
func embeddingModel(raw []byte) string {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return unknownEmbeddingModel
	}
	if strings.TrimSpace(payload.Model) == "" {
		return unknownEmbeddingModel
	}
	return payload.Model
}

// embeddingUpstreamURL joins the provider base URL with the embeddings route and request query.
func embeddingUpstreamURL(baseURL string, rawQuery string) (string, error) {
	upstream, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return "", fmt.Errorf("parse embeddings upstream URL: %w", err)
	}
	requestPath, err := url.JoinPath(upstream.Path, routeEmbeddings)
	if err != nil {
		return "", fmt.Errorf("join embeddings upstream path: %w", err)
	}
	if requestPath == "" || requestPath[0] != '/' {
		requestPath = "/" + requestPath
	}
	upstream.Path = requestPath
	upstream.RawPath = ""
	upstream.RawQuery = rawQuery
	return upstream.String(), nil
}

// copyResponseHeaders copies safe upstream response headers to the downstream response.
func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		if _, excluded := responseHeadersExcludedFromCopy[http.CanonicalHeaderKey(key)]; excluded {
			continue
		}
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
