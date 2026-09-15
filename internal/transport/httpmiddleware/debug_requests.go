package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
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

// DebugRequests logs complete incoming proxy requests while leaving the consumed body stream unchanged.
func (w *JSONLineWriter) DebugRequests(maxBodyBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
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
			capture := &boundedCapture{limit: maxBodyBytes}
			body := request.Body
			if body != nil {
				body = &mirroredReadCloser{Reader: io.TeeReader(body, capture), Closer: body}
				request.Body = body
			}
			defer func() {
				if body != nil {
					_, _ = io.Copy(io.Discard, body)
				}
				_ = w.writeRequest(info, capture)
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
	data      []byte
	limit     int64
	truncated bool
}

func (capture *boundedCapture) Write(data []byte) (int, error) {
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

func (w *JSONLineWriter) writeRequest(info debugRequestInfo, capture *boundedCapture) error {
	event := map[string]any{
		"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
		"level":       "DEBUG",
		"event":       debugRequestEvent,
		"method":      info.method,
		"scheme":      info.scheme,
		"host":        info.host,
		"path":        info.path,
		"raw_query":   info.rawQuery,
		"protocol":    info.protocol,
		"remote_addr": info.remoteAddr,
		"headers":     info.headers,
		"body":        debugRequestBody(capture.data, capture.truncated),
	}
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
