package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"

	"promptgate/backend/internal/platform/secrets"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newTestService creates an in-memory MCP service with a test cipher.
func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	service := NewService(db, cipher)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return service, db
}

// headerValue builds a present header value for service tests.
func headerValue(value string) HeaderValue {
	return HeaderValue{Set: true, Value: &value}
}

// TestMCPHeadersEncryptSensitiveAndExposePlainHeaders verifies sensitive headers are encrypted and redacted.
func TestMCPHeadersEncryptSensitiveAndExposePlainHeaders(t *testing.T) {
	service, db := newTestService(t)
	ctx := context.Background()

	resp, err := service.CreateServer(ctx, CreateServerInput{
		Name: "deepwiki",
		URL:  "https://mcp.example.com/mcp",
		Headers: []HeaderInput{
			{Name: "Authorization", Value: headerValue("Bearer secret"), Sensitive: true},
			{Name: "X-Trace", Value: headerValue("visible"), Sensitive: false},
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	if len(resp.Headers) != 2 {
		t.Fatalf("expected 2 headers, got %d", len(resp.Headers))
	}
	if resp.Headers[0].Sensitive && resp.Headers[0].Value != "" && resp.Headers[0].Name == "Authorization" {
		t.Fatal("sensitive header leaked value")
	}

	var record MCPServer
	if err := db.First(&record, "id = ?", resp.ID).Error; err != nil {
		t.Fatalf("load server: %v", err)
	}
	if record.Headers[0].ValueCiphertext == "" || record.Headers[0].Value != "" {
		t.Fatalf("expected encrypted sensitive header, got %#v", record.Headers[0])
	}

	headers, err := service.HeadersForProxy(record)
	if err != nil {
		t.Fatalf("headers for proxy: %v", err)
	}
	if headers["Authorization"] != "Bearer secret" {
		t.Fatalf("expected decrypted auth header, got %q", headers["Authorization"])
	}
	if headers["X-Trace"] != "visible" {
		t.Fatalf("expected visible trace header, got %q", headers["X-Trace"])
	}
}

// TestUpdateMCPHeaderPreservesSensitiveValueWhenValueAbsent verifies omitted sensitive values are preserved.
func TestUpdateMCPHeaderPreservesSensitiveValueWhenValueAbsent(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	created, err := service.CreateServer(ctx, CreateServerInput{
		Name: "deepwiki",
		URL:  "https://mcp.example.com/mcp",
		Headers: []HeaderInput{
			{Name: "Authorization", Value: headerValue("Bearer secret"), Sensitive: true},
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	updated, err := service.UpdateServer(ctx, created.ID.String(), UpdateServerInput{
		Headers: &[]HeaderInput{
			{Name: "Authorization", Sensitive: true},
		},
	})
	if err != nil {
		t.Fatalf("update server: %v", err)
	}
	if !updated.Headers[0].HasValue || updated.Headers[0].Value != "" {
		t.Fatalf("expected preserved redacted sensitive value, got %#v", updated.Headers[0])
	}

	var record MCPServer
	if err := service.db.First(&record, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("load server: %v", err)
	}
	headers, err := service.HeadersForProxy(record)
	if err != nil {
		t.Fatalf("headers for proxy: %v", err)
	}
	if headers["Authorization"] != "Bearer secret" {
		t.Fatalf("expected preserved secret, got %q", headers["Authorization"])
	}
}
