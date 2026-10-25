package subscriptions

import (
	"bytes"
	"encoding/json"
	"sync"
)

// responsesSSEUsageObserver inspects a bounded copy of individual Responses
// API SSE events while the original bytes continue to stream to the caller.
// If an event exceeds the bound, observation stops and the quota middleware
// keeps the conservative reservation instead.
type responsesSSEUsageObserver struct {
	state    *quotaRequestState
	maxBytes int64

	mu       sync.Mutex
	buffer   []byte
	disabled bool
}

func newResponsesSSEUsageObserver(state *quotaRequestState, maxBytes int64) *responsesSSEUsageObserver {
	return &responsesSSEUsageObserver{state: state, maxBytes: maxBytes}
}

func (o *responsesSSEUsageObserver) Write(raw []byte) {
	if o == nil || len(raw) == 0 {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.disabled {
		return
	}
	if o.maxBytes <= 0 || int64(len(o.buffer))+int64(len(raw)) > o.maxBytes {
		o.buffer = nil
		o.disabled = true
		return
	}
	o.buffer = append(o.buffer, raw...)
	for {
		end := sseEventEnd(o.buffer)
		if end < 0 {
			return
		}
		o.observeEvent(o.buffer[:end])
		o.buffer = append(o.buffer[:0], o.buffer[end:]...)
		if o.disabled {
			o.buffer = nil
			return
		}
	}
}

func sseEventEnd(raw []byte) int {
	lineStart := 0
	for lineStart < len(raw) {
		relativeEnd := bytes.IndexByte(raw[lineStart:], '\n')
		if relativeEnd < 0 {
			return -1
		}
		lineEnd := lineStart + relativeEnd
		contentEnd := lineEnd
		if contentEnd > lineStart && raw[contentEnd-1] == '\r' {
			contentEnd--
		}
		if contentEnd == lineStart {
			return lineEnd + 1
		}
		lineStart = lineEnd + 1
	}
	return -1
}

func (o *responsesSSEUsageObserver) observeEvent(raw []byte) {
	var data []byte
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		part := bytes.TrimPrefix(line, []byte("data:"))
		part = bytes.TrimPrefix(part, []byte{' '})
		if len(data) > 0 {
			data = append(data, '\n')
		}
		data = append(data, part...)
	}
	if len(data) == 0 {
		return
	}
	if bytes.Equal(data, []byte("[DONE]")) {
		o.state.markStreamCompleted()
		o.disabled = true
		return
	}

	var event struct {
		Type     string `json:"type"`
		Response struct {
			Usage *struct {
				InputTokens  int64 `json:"input_tokens"`
				OutputTokens int64 `json:"output_tokens"`
				TotalTokens  int64 `json:"total_tokens"`
			} `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return
	}
	if event.Type == "message_stop" {
		o.state.markStreamCompleted()
		o.disabled = true
		return
	}
	if event.Type != "response.completed" || event.Response.Usage == nil {
		return
	}
	tokens := event.Response.Usage.TotalTokens
	if tokens <= 0 {
		tokens = saturatingAdd(event.Response.Usage.InputTokens, event.Response.Usage.OutputTokens)
	}
	if tokens > 0 {
		o.state.setResponseTokens(tokens)
		o.state.markStreamCompleted()
		o.disabled = true
	}
}
