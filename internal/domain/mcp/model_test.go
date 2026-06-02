package mcp

import "testing"

// TestMCPHeadersScanLegacyBooleanMap verifies MCP headers scan legacy boolean map.
func TestMCPHeadersScanLegacyBooleanMap(t *testing.T) {
	var headers MCPHeaders
	if err := headers.Scan([]byte(`{"Authorization":true,"X-Trace":"visible"}`)); err != nil {
		t.Fatalf("scan headers: %v", err)
	}

	auth := headerByName(headers, "Authorization")
	if auth.Name == "" {
		t.Fatal("expected Authorization header")
	}
	if !auth.Sensitive || auth.Value != "" {
		t.Fatalf("expected sensitive Authorization without value, got %#v", auth)
	}

	trace := headerByName(headers, "X-Trace")
	if trace.Value != "visible" || trace.Sensitive {
		t.Fatalf("expected visible X-Trace header, got %#v", trace)
	}
}

// TestMCPHeadersScanStructuredBooleanValue verifies MCP headers scan structured boolean value.
func TestMCPHeadersScanStructuredBooleanValue(t *testing.T) {
	var headers MCPHeaders
	if err := headers.Scan([]byte(`[{"name":"X-Flag","value":true,"sensitive":false}]`)); err != nil {
		t.Fatalf("scan headers: %v", err)
	}

	flag := headerByName(headers, "X-Flag")
	if flag.Name == "" {
		t.Fatal("expected X-Flag header")
	}
	if flag.Value != "" || flag.Sensitive {
		t.Fatalf("expected bool value to be ignored for X-Flag, got %#v", flag)
	}
}

// headerByName returns header by name.
func headerByName(headers MCPHeaders, name string) MCPHeader {
	for _, header := range headers {
		if header.Name == name {
			return header
		}
	}
	return MCPHeader{}
}
