package runtime

import "net/http"

// upstreamHeaderAllowlist contains the small set of client headers that are
// part of supported provider request contracts. Authentication, cookies,
// forwarding metadata, tracing baggage, native session identifiers, internal
// administration headers, and unknown custom headers are intentionally absent.
var upstreamHeaderAllowlist = map[string]struct{}{
	"Accept":            {},
	"Accept-Encoding":   {},
	"Anthropic-Beta":    {},
	"Anthropic-Version": {},
	"Content-Type":      {},
	"Openai-Beta":       {},
}

func sanitizeUpstreamHeaders(headers http.Header) http.Header {
	sanitized := make(http.Header, len(upstreamHeaderAllowlist))
	for name, values := range headers {
		canonicalName := http.CanonicalHeaderKey(name)
		if _, allowed := upstreamHeaderAllowlist[canonicalName]; !allowed {
			continue
		}
		sanitized[canonicalName] = append([]string(nil), values...)
	}
	return sanitized
}
