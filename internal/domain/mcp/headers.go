package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// HeaderValue preserves the tri-state distinction between an omitted value,
// an explicit null (clear), and a supplied string.
type HeaderValue struct {
	Set   bool
	Value *string
}

func (v *HeaderValue) UnmarshalJSON(data []byte) error {
	v.Set = true
	if bytes.Equal(data, []byte("null")) {
		v.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	v.Value = &value
	return nil
}

type HeaderInput struct {
	Name      string      `json:"name"`
	Value     HeaderValue `json:"value"`
	Sensitive bool        `json:"sensitive"`
}

type HeaderResponse struct {
	Name      string `json:"name"`
	Value     string `json:"value,omitempty"`
	Sensitive bool   `json:"sensitive"`
	HasValue  bool   `json:"hasValue"`
}

// HeadersForProxy decrypts and returns headers needed by the proxy.
func (s *Service) HeadersForProxy(server MCPServer) (map[string]string, error) {
	headers := make(map[string]string, len(server.Headers))
	for _, header := range server.Headers {
		value := header.Value
		if header.Sensitive && header.ValueCiphertext != "" {
			plain, err := s.cipher.Decrypt(header.ValueCiphertext)
			if err != nil {
				return nil, fmt.Errorf("decrypt mcp header %q: %w", header.Name, err)
			}
			value = plain
		}
		if value != "" {
			headers[header.Name] = value
		}
	}
	return headers, nil
}

func (s *Service) buildHeaders(inputs []HeaderInput, existing MCPHeaders) (MCPHeaders, error) {
	existingByName := make(map[string]MCPHeader, len(existing))
	for _, header := range existing {
		existingByName[strings.ToLower(header.Name)] = header
	}

	out := make(MCPHeaders, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		name := httpHeaderName(input.Name)
		if name == "" {
			return nil, ErrInvalidHeader
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return nil, ErrInvalidHeader
		}
		seen[key] = struct{}{}

		value, err := s.headerValue(input, existingByName[key])
		if err != nil {
			return nil, err
		}
		header := MCPHeader{Name: name, Sensitive: input.Sensitive}
		if input.Sensitive {
			if value != "" {
				ciphertext, err := s.cipher.Encrypt(value)
				if err != nil {
					return nil, err
				}
				header.ValueCiphertext = ciphertext
			}
		} else {
			header.Value = value
		}
		out = append(out, header)
	}
	return out, nil
}

func (s *Service) headerValue(input HeaderInput, existing MCPHeader) (string, error) {
	if input.Value.Set {
		if input.Value.Value == nil {
			return "", nil
		}
		return strings.TrimSpace(*input.Value.Value), nil
	}
	if existing.Name == "" {
		return "", nil
	}
	if existing.Sensitive {
		if existing.ValueCiphertext == "" {
			return "", nil
		}
		return s.cipher.Decrypt(existing.ValueCiphertext)
	}
	return existing.Value, nil
}

func (s *Service) toResponse(record MCPServer) ServerResponse {
	headers := make([]HeaderResponse, 0, len(record.Headers))
	for _, header := range record.Headers {
		response := HeaderResponse{
			Name:      header.Name,
			Sensitive: header.Sensitive,
			HasValue:  header.Value != "" || header.ValueCiphertext != "",
		}
		if !header.Sensitive {
			response.Value = header.Value
		}
		headers = append(headers, response)
	}
	return ServerResponse{
		ID:           record.ID,
		Name:         record.Name,
		DisplayName:  record.DisplayName,
		URL:          record.URL,
		Headers:      headers,
		AllowPattern: record.AllowPattern,
		DenyPattern:  record.DenyPattern,
		Enabled:      record.Enabled,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}
}

func httpHeaderName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" || strings.ContainsAny(name, " \t\r\n:") {
		return ""
	}
	return name
}
