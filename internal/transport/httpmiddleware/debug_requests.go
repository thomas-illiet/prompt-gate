package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"promptgate/backend/internal/platform/requestprivacy"
)

const debugRequestEvent = "proxy_request_debug"

// JSONLineWriter serializes structured diagnostic events as one JSON object per line.
type JSONLineWriter struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

// NewJSONLineWriter builds a concurrency-safe JSON-lines writer.
func NewJSONLineWriter(output io.Writer) *JSONLineWriter {
	return &JSONLineWriter{encoder: json.NewEncoder(output)}
}

// WriteStartupWarning emits the explicit sensitive-data warning for request debugging.
func (w *JSONLineWriter) WriteStartupWarning() error {
	return w.write(map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"level":     "WARN",
		"event":     "proxy_request_debug_enabled",
		"message":   "proxy request debugging is enabled; passwords, JWTs, cookies, provider keys, prompts, and personal data will be written to stdout in cleartext",
	})
}

// DebugRequests logs request metadata and body bytes consumed by downstream handlers.
func (w *JSONLineWriter) DebugRequests(maxBodyBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			privacyContext := request.Context()
			info := debugRequestInfo{
				method:     request.Method,
				scheme:     requestScheme(request),
				host:       request.Host,
				path:       request.URL.Path,
				rawQuery:   request.URL.RawQuery,
				protocol:   request.Proto,
				remoteAddr: request.RemoteAddr,
				headers:    request.Header.Clone(),
			}
			capture := &boundedCapture{
				limit: maxBodyBytes,
				contentAllowed: func() bool {
					return !requestprivacy.IsMetadataOnly(privacyContext)
				},
			}
			body := request.Body
			if body != nil {
				body = &mirroredReadCloser{Reader: io.TeeReader(body, capture), Closer: body}
				request.Body = body
			}
			defer func() {
				_ = w.writeRequest(info, capture, requestprivacy.IsMetadataOnly(privacyContext))
			}()

			next.ServeHTTP(response, request)
		})
	}
}

type debugRequestInfo struct {
	method     string
	scheme     string
	host       string
	path       string
	rawQuery   string
	protocol   string
	remoteAddr string
	headers    http.Header
}

type mirroredReadCloser struct {
	io.Reader
	io.Closer
}

type boundedCapture struct {
	data           []byte
	limit          int64
	truncated      bool
	contentAllowed func() bool
}

func (capture *boundedCapture) Write(data []byte) (int, error) {
	if capture.contentAllowed != nil && !capture.contentAllowed() {
		capture.data = nil
		capture.truncated = false
		return len(data), nil
	}
	remaining := capture.limit - int64(len(capture.data))
	if remaining > 0 {
		kept := int64(len(data))
		if kept > remaining {
			kept = remaining
		}
		capture.data = append(capture.data, data[:kept]...)
	}
	if int64(len(data)) > remaining {
		capture.truncated = true
	}
	return len(data), nil
}

func (w *JSONLineWriter) writeRequest(info debugRequestInfo, capture *boundedCapture, metadataOnly bool) error {
	event := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"level":     "DEBUG",
		"event":     debugRequestEvent,
		"method":    info.method,
		"path":      info.path,
		"protocol":  info.protocol,
	}
	if metadataOnly {
		event["privacy_mode"] = "metadata_only"
		return w.write(event)
	}
	event["scheme"] = info.scheme
	event["host"] = info.host
	event["remote_addr"] = info.remoteAddr
	event["raw_query"] = info.rawQuery
	event["headers"] = info.headers
	event["body"] = debugRequestBody(capture.data, capture.truncated)
	if capture.truncated {
		event["body_truncated"] = true
	}
	return w.write(event)
}

func (w *JSONLineWriter) write(event any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.encoder.Encode(event)
}

func requestScheme(request *http.Request) string {
	if request.URL.Scheme != "" {
		return request.URL.Scheme
	}
	if request.TLS != nil {
		return "https"
	}
	return "http"
}

func debugRequestBody(data []byte, truncated bool) any {
	if truncated {
		return string(data)
	}
	var decoded any
	if json.Unmarshal(data, &decoded) == nil {
		switch decoded.(type) {
		case map[string]any, []any:
			return decoded
		}
	}
	return string(data)
}
