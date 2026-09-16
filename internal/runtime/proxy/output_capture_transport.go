package runtime

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
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

// NewOutputCaptureTransport attaches assistant text to the active provider span
// without buffering or delaying the response delivered to AIBridge.
func NewOutputCaptureTransport(base http.RoundTripper, maxBytes int64) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &outputCaptureTransport{base: base, maxBytes: maxBytes}
}

type outputCaptureTransport struct {
	base     http.RoundTripper
	maxBytes int64
}

func (t *outputCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, err
	}
	span := trace.SpanFromContext(req.Context())
	if !span.IsRecording() {
		return resp, nil
	}
	resp.Body = &capturingResponseBody{
		ReadCloser:  resp.Body,
		span:        span,
		contentType: resp.Header.Get("Content-Type"),
		maxBytes:    t.maxBytes,
	}
	return resp, nil
}

type capturingResponseBody struct {
	io.ReadCloser
	span        trace.Span
	contentType string
	maxBytes    int64
	buffer      bytes.Buffer
	overflow    bool
	once        sync.Once
}

func (b *capturingResponseBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 && !b.overflow {
		remaining := b.maxBytes - int64(b.buffer.Len())
		if b.maxBytes <= 0 || int64(n) > remaining {
			b.overflow = true
			b.buffer.Reset()
		} else {
			_, _ = b.buffer.Write(p[:n])
		}
	}
	if err == io.EOF {
		b.publish()
	}
	return n, err
}

func (b *capturingResponseBody) publish() {
	b.once.Do(func() {
		if b.overflow {
			return
		}
		output := extractAssistantOutput(b.buffer.Bytes(), b.contentType)
		if output == "" {
			return
		}
		b.span.SetAttributes(
			attribute.String(openInferenceOutputValue, output),
			attribute.String(openInferenceOutputMIMEType, "text/plain"),
		)
	})
}

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
	// Provider events can contain large text deltas; the outer capture limit is
	// the authoritative bound.
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
