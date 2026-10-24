package modellab

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestValidateAndNormalizePayloadAcceptsTextChatHistory(t *testing.T) {
	tests := []struct {
		name     string
		protocol Protocol
		payload  string
		messages []generationMessage
	}{
		{
			name:     "openai one turn remains compatible",
			protocol: ProtocolOpenAIChat,
			payload:  `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":true,"temperature":0.4,"max_tokens":512}`,
			messages: []generationMessage{{Role: "user", Content: "hello"}},
		},
		{
			name:     "openai multi turn",
			protocol: ProtocolOpenAIChat,
			payload:  `{"model":"gpt-5","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"},{"role":"user","content":"three"}],"stream":true,"temperature":0.4,"max_tokens":512}`,
			messages: []generationMessage{
				{Role: "user", Content: "one"},
				{Role: "assistant", Content: "two"},
				{Role: "user", Content: "three"},
			},
		},
		{
			name:     "anthropic multi turn",
			protocol: ProtocolAnthropicMessages,
			payload:  `{"model":"gpt-5","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"  two  "},{"role":"user","content":"three"}],"stream":true,"temperature":0.4,"max_tokens":512}`,
			messages: []generationMessage{
				{Role: "user", Content: "one"},
				{Role: "assistant", Content: "  two  "},
				{Role: "user", Content: "three"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			normalized, err := ValidateAndNormalizePayload(test.protocol, "gpt-5", []byte(test.payload), DefaultMaxRequestBytes, 4096)
			if err != nil {
				t.Fatalf("validate payload: %v", err)
			}
			var decoded generationRequest
			if err := json.Unmarshal(normalized, &decoded); err != nil {
				t.Fatalf("decode normalized payload: %v", err)
			}
			if !reflect.DeepEqual(decoded.Messages, test.messages) {
				t.Fatalf("unexpected normalized messages: %#v", decoded.Messages)
			}
			if decoded.MaxTokens == nil || *decoded.MaxTokens != 512 || decoded.Temperature == nil || *decoded.Temperature != 0.4 {
				t.Fatalf("unexpected normalized settings: %#v", decoded)
			}
		})
	}
}

func TestValidateAndNormalizePayloadRejectsInvalidChatHistory(t *testing.T) {

	cases := map[string]string{
		"no messages":            `{"model":"gpt-5","messages":[],"stream":true}`,
		"null messages":          `{"model":"gpt-5","messages":null,"stream":true}`,
		"assistant first":        `{"model":"gpt-5","messages":[{"role":"assistant","content":"hello"},{"role":"user","content":"next"}],"stream":true}`,
		"assistant last":         `{"model":"gpt-5","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}],"stream":true}`,
		"consecutive users":      `{"model":"gpt-5","messages":[{"role":"user","content":"one"},{"role":"user","content":"two"},{"role":"user","content":"three"}],"stream":true}`,
		"consecutive assistants": `{"model":"gpt-5","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"},{"role":"assistant","content":"three"}],"stream":true}`,
		"empty user":             `{"model":"gpt-5","messages":[{"role":"user","content":"  "}],"stream":true}`,
		"empty assistant":        `{"model":"gpt-5","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"\n\t"},{"role":"user","content":"three"}],"stream":true}`,
		"system":                 `{"model":"gpt-5","messages":[{"role":"system","content":"secret"}],"stream":true}`,
		"message name":           `{"model":"gpt-5","messages":[{"role":"user","content":"hello","name":"user"}],"stream":true}`,
		"wrong model":            `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}],"stream":true}`,
		"not streaming":          `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":false}`,
		"tools":                  `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":true,"tools":[]}`,
		"multimodal":             `{"model":"gpt-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"stream":true}`,
		"temperature":            `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":true,"temperature":2}`,
		"tokens":                 `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":999999}`,
		"trailing":               `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":true}{}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateAndNormalizePayload(ProtocolOpenAIChat, "gpt-5", []byte(payload), DefaultMaxRequestBytes, 4096); !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("expected invalid payload, got %v", err)
			}
		})
	}

	valid := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	if _, err := ValidateAndNormalizePayload(Protocol("unsupported"), "gpt-5", valid, DefaultMaxRequestBytes, 4096); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected unsupported protocol rejection, got %v", err)
	}
}

func TestValidateAndNormalizePayloadEnforcesSizeAndDefaults(t *testing.T) {
	payload := `{"model":"claude-sonnet","messages":[{"role":"user","content":"hello"}],"stream":true}`
	normalized, err := ValidateAndNormalizePayload(ProtocolAnthropicMessages, "claude-sonnet", []byte(payload), DefaultMaxRequestBytes, 4096)
	if err != nil {
		t.Fatalf("validate anthropic payload: %v", err)
	}
	if !strings.Contains(string(normalized), `"max_tokens":1024`) {
		t.Fatalf("expected default max tokens, got %s", normalized)
	}
	if _, err := ValidateAndNormalizePayload(ProtocolAnthropicMessages, "claude-sonnet", []byte(payload), 8, 4096); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected size rejection, got %v", err)
	}
	bounded, err := ValidateAndNormalizePayload(ProtocolAnthropicMessages, "claude-sonnet", []byte(payload), DefaultMaxRequestBytes, 512)
	if err != nil || !strings.Contains(string(bounded), `"max_tokens":512`) {
		t.Fatalf("expected configured default token cap, payload=%s err=%v", bounded, err)
	}
	overLimit := `{"model":"claude-sonnet","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":513}`
	if _, err := ValidateAndNormalizePayload(ProtocolAnthropicMessages, "claude-sonnet", []byte(overLimit), DefaultMaxRequestBytes, 512); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected configured output-token rejection, got %v", err)
	}
}
