package mcp

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MCPHeader struct {
	Name            string `json:"name"`
	Value           string `json:"value,omitempty"`
	ValueCiphertext string `json:"valueCiphertext,omitempty"`
	Sensitive       bool   `json:"sensitive"`
}

type MCPHeaders []MCPHeader

// Value serializes MCP headers as JSON for database storage.
func (h MCPHeaders) Value() (driver.Value, error) {
	b, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan deserializes MCP headers from database storage.
func (h *MCPHeaders) Scan(v interface{}) error {
	var data []byte
	switch val := v.(type) {
	case []byte:
		data = val
	case string:
		data = []byte(val)
	case nil:
		*h = nil
		return nil
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}

	if headers, ok := scanStructuredHeaders(data); ok {
		*h = headers
		return nil
	}

	var legacy map[string]json.RawMessage
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	headers := make(MCPHeaders, 0, len(legacy))
	for name, raw := range legacy {
		header := MCPHeader{Name: name}
		value, sensitive := scanHeaderValue(raw)
		header.Value = value
		header.Sensitive = sensitive
		headers = append(headers, header)
	}
	*h = headers
	return nil
}

type scannedMCPHeader struct {
	Name            string          `json:"name"`
	Value           json.RawMessage `json:"value"`
	ValueCiphertext string          `json:"valueCiphertext"`
	Sensitive       bool            `json:"sensitive"`
}

// scanStructuredHeaders decodes the current structured MCP header JSON format.
func scanStructuredHeaders(data []byte) (MCPHeaders, bool) {
	var scanned []scannedMCPHeader
	if err := json.Unmarshal(data, &scanned); err != nil {
		return nil, false
	}

	headers := make(MCPHeaders, 0, len(scanned))
	for _, raw := range scanned {
		value, _ := scanHeaderValue(raw.Value)
		headers = append(headers, MCPHeader{
			Name:            raw.Name,
			Value:           value,
			ValueCiphertext: raw.ValueCiphertext,
			Sensitive:       raw.Sensitive,
		})
	}
	return headers, true
}

// scanHeaderValue decodes legacy and structured MCP header value shapes.
func scanHeaderValue(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}

	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, false
	}

	var sensitive bool
	if err := json.Unmarshal(raw, &sensitive); err == nil {
		return "", sensitive
	}

	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String(), false
	}

	return strconv.Quote(string(raw)), false
}

type MCPServer struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name         string     `gorm:"not null;uniqueIndex" json:"name"`
	DisplayName  string     `gorm:"not null;default:''" json:"displayName"`
	URL          string     `gorm:"not null" json:"url"`
	Headers      MCPHeaders `gorm:"type:jsonb;default:'[]'" json:"headers"`
	AllowPattern string     `gorm:"not null;default:''" json:"allowPattern"`
	DenyPattern  string     `gorm:"not null;default:''" json:"denyPattern"`
	Enabled      bool       `gorm:"not null;default:true" json:"enabled"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// BeforeCreate assigns ids and normalized names before MCP insertion.
func (s *MCPServer) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.Name = strings.TrimSpace(strings.ToLower(s.Name))
	return nil
}

// BeforeUpdate normalizes MCP names before updates.
func (s *MCPServer) BeforeUpdate(_ *gorm.DB) error {
	s.Name = strings.TrimSpace(strings.ToLower(s.Name))
	return nil
}
