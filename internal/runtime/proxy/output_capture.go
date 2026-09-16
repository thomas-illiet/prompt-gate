package runtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	openInferenceOutputValue    = "output.value"
	openInferenceOutputMIMEType = "output.mime_type"
)

type outputCaptureContextKey struct{}

type outputCapture struct {
	mu          sync.Mutex
	span        trace.Span
	statusCode  int
	contentType string
	maxBytes    int64
	buffer      bytes.Buffer
	overflow    bool
	published   bool
}

// OutputCaptureMiddleware observes the client-visible provider response without
// delaying writes. The buffered body is bounded and only enabled by explicit
// output-capture consent.
func OutputCaptureMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capture := &outputCapture{maxBytes: maxBytes}
			ctx := context.WithValue(r.Context(), outputCaptureContextKey{}, capture)
			next.ServeHTTP(&outputCaptureResponseWriter{ResponseWriter: w, capture: capture}, r.WithContext(ctx))
		})
	}
}

// BindOutputSpan attaches the main interception span to the response observer.
func BindOutputSpan(ctx context.Context, span trace.Span) {
	capture := outputCaptureFromContext(ctx)
	if capture == nil {
		return
	}
	capture.mu.Lock()
	capture.span = span
	capture.mu.Unlock()
}

// PublishOutput parses and attaches the complete response while the main
// interception span is still recording.
func PublishOutput(ctx context.Context) {
	capture := outputCaptureFromContext(ctx)
	if capture == nil {
		return
	}
	capture.publish()
}

func outputCaptureFromContext(ctx context.Context) *outputCapture {
	if ctx == nil {
		return nil
	}
	capture, _ := ctx.Value(outputCaptureContextKey{}).(*outputCapture)
	return capture
}

func (c *outputCapture) writeHeader(statusCode int, contentType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.statusCode != 0 {
		return
	}
	c.statusCode = statusCode
	c.contentType = contentType
}

func (c *outputCapture) write(body []byte, contentType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.statusCode == 0 {
		c.statusCode = http.StatusOK
		c.contentType = contentType
	}
	if c.overflow || len(body) == 0 {
		return
	}
	remaining := c.maxBytes - int64(c.buffer.Len())
	if c.maxBytes <= 0 || int64(len(body)) > remaining {
		c.overflow = true
		c.buffer.Reset()
		return
	}
	_, _ = c.buffer.Write(body)
}

func (c *outputCapture) publish() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.published {
		return
	}
	c.published = true
	if c.span == nil || !c.span.IsRecording() || c.overflow || c.statusCode < 200 || c.statusCode >= 300 {
		return
	}
	output := extractAssistantOutput(c.buffer.Bytes(), c.contentType)
	if output == "" {
		return
	}
	c.span.SetAttributes(
		attribute.String(openInferenceOutputValue, output),
		attribute.String(openInferenceOutputMIMEType, "text/plain"),
	)
}

type outputCaptureResponseWriter struct {
	http.ResponseWriter
	capture *outputCapture
}

func (w *outputCaptureResponseWriter) WriteHeader(statusCode int) {
	w.capture.writeHeader(statusCode, w.Header().Get("Content-Type"))
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *outputCaptureResponseWriter) Write(body []byte) (int, error) {
	w.capture.write(body, w.Header().Get("Content-Type"))
	return w.ResponseWriter.Write(body)
}

func (w *outputCaptureResponseWriter) Flush() {
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *outputCaptureResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *outputCaptureResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{w}, reader)
}

func (w *outputCaptureResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func extractAssistantOutput(raw []byte, contentType string) string {
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return extractSSEOutput(raw)
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	return extractJSONOutput(payload)
}

func extractJSONOutput(payload map[string]any) string {
	var parts []string
	if choices, ok := payload["choices"].([]any); ok {
		for _, choice := range choices {
			choiceMap, _ := choice.(map[string]any)
			message, _ := choiceMap["message"].(map[string]any)
			parts = appendTextContent(parts, message["content"])
		}
	}
	if content, ok := payload["content"].([]any); ok {
		parts = appendTypedText(parts, content, "text")
	}
	if output, ok := payload["output"].([]any); ok {
		for _, item := range output {
			itemMap, _ := item.(map[string]any)
			if content, ok := itemMap["content"].([]any); ok {
				parts = appendTypedText(parts, content, "output_text")
			}
		}
	}
	return strings.Join(parts, "")
}

func appendTextContent(parts []string, content any) []string {
	switch value := content.(type) {
	case string:
		return append(parts, value)
	case []any:
		return appendTypedText(parts, value, "text")
	default:
		return parts
	}
}

func appendTypedText(parts []string, content []any, allowedType string) []string {
	for _, part := range content {
		partMap, _ := part.(map[string]any)
		partType, _ := partMap["type"].(string)
		text, _ := partMap["text"].(string)
		if partType == allowedType && text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}

func extractSSEOutput(raw []byte) string {
	var output strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), len(raw)+1)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event map[string]any
		if json.Unmarshal([]byte(data), &event) != nil {
			continue
		}
		if eventType, _ := event["type"].(string); eventType == "response.output_text.delta" {
			delta, _ := event["delta"].(string)
			output.WriteString(delta)
			continue
		}
		if delta, ok := event["delta"].(map[string]any); ok {
			if deltaType, _ := delta["type"].(string); deltaType == "text_delta" {
				text, _ := delta["text"].(string)
				output.WriteString(text)
				continue
			}
		}
		if choices, ok := event["choices"].([]any); ok {
			for _, choice := range choices {
				choiceMap, _ := choice.(map[string]any)
				delta, _ := choiceMap["delta"].(map[string]any)
				text, _ := delta["content"].(string)
				output.WriteString(text)
			}
		}
	}
	return output.String()
}
