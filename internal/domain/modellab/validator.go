package modellab

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const (
	DefaultMaxRequestBytes       = 32 << 10
	defaultRequestedOutputTokens = 1024
)

type generationRequest struct {
	Model       string              `json:"model"`
	Messages    []generationMessage `json:"messages"`
	Stream      bool                `json:"stream"`
	Temperature *float64            `json:"temperature,omitempty"`
	MaxTokens   *int                `json:"max_tokens,omitempty"`
}

type generationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ValidateAndNormalizePayload applies this service's configured output-token
// ceiling while keeping the request-size limit specific to the proxy runtime.
func (s *Service) ValidateAndNormalizePayload(protocol Protocol, expectedModel string, raw []byte, maxBytes int) ([]byte, error) {
	if s == nil {
		return nil, ErrInvalidPayload
	}
	return ValidateAndNormalizePayload(protocol, expectedModel, raw, maxBytes, s.maxOutputTokens)
}

// ValidateAndNormalizePayload rejects every feature outside the text-only chat
// contract and returns a freshly encoded body for the upstream provider.
// Re-encoding prevents differences between the validator and provider JSON
// parsers from turning duplicate or ambiguous fields into a policy bypass.
func ValidateAndNormalizePayload(protocol Protocol, expectedModel string, raw []byte, maxBytes, maxOutputTokens int) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxRequestBytes
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultMaxOutputTokens
	}
	if len(raw) == 0 || len(raw) > maxBytes {
		return nil, ErrInvalidPayload
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var input generationRequest
	if err := decoder.Decode(&input); err != nil {
		return nil, ErrInvalidPayload
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidPayload
	}

	if input.Model != expectedModel || !input.Stream || len(input.Messages) == 0 {
		return nil, ErrInvalidPayload
	}
	canonicalMessages := make([]generationMessage, len(input.Messages))
	for index, message := range input.Messages {
		expectedRole := "user"
		if index%2 == 1 {
			expectedRole = "assistant"
		}
		if message.Role != expectedRole || strings.TrimSpace(message.Content) == "" {
			return nil, ErrInvalidPayload
		}
		canonicalMessages[index] = generationMessage{
			Role:    expectedRole,
			Content: message.Content,
		}
	}
	if canonicalMessages[len(canonicalMessages)-1].Role != "user" {
		return nil, ErrInvalidPayload
	}
	if input.Temperature != nil && (*input.Temperature < 0 || *input.Temperature > 1) {
		return nil, ErrInvalidPayload
	}
	maxTokens := min(defaultRequestedOutputTokens, maxOutputTokens)
	if input.MaxTokens != nil {
		maxTokens = *input.MaxTokens
	}
	if maxTokens < 1 || maxTokens > maxOutputTokens {
		return nil, ErrInvalidPayload
	}
	if protocol != ProtocolOpenAIChat && protocol != ProtocolAnthropicMessages {
		return nil, ErrInvalidPayload
	}

	canonical := generationRequest{
		Model:       input.Model,
		Messages:    canonicalMessages,
		Stream:      true,
		Temperature: input.Temperature,
		MaxTokens:   &maxTokens,
	}
	normalized, err := json.Marshal(canonical)
	if err != nil {
		return nil, ErrInvalidPayload
	}
	return normalized, nil
}
