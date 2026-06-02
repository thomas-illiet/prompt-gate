package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/provider"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newProvidersTestHandler creates provider test handler.
func newProvidersTestHandler(t *testing.T) (*Handler, *provider.Service) {
	t.Helper()

	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	service := provider.NewService(db, nil)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate provider table: %v", err)
	}

	return NewHandler(nil, nil, nil, nil, service, nil), service
}

// TestHandleAdminUpdateProviderRejectsName verifies provider route names are immutable through PATCH.
func TestHandleAdminUpdateProviderRejectsName(t *testing.T) {
	handler, service := newProvidersTestHandler(t)
	ctx := context.Background()
	created, err := service.CreateProvider(ctx, provider.CreateProviderInput{
		Name:        "openai-main",
		DisplayName: "OpenAI Main",
		Type:        provider.ProviderTypeOpenAI,
		BaseURL:     "https://api.openai.com/v1",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/providers/"+created.ID.String(),
		bytes.NewBufferString(`{"name":"openai-renamed","displayName":"OpenAI Primary"}`),
	)
	req.SetPathValue("id", created.ID.String())
	recorder := httptest.NewRecorder()

	handler.HandleAdminUpdateProvider(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "invalid_request_body" {
		t.Fatalf("unexpected error body: %#v", body)
	}

	reloaded, err := service.GetProvider(ctx, created.ID.String())
	if err != nil {
		t.Fatalf("reload provider: %v", err)
	}
	if reloaded.Name != "openai-main" || reloaded.DisplayName != "OpenAI Main" {
		t.Fatalf("unexpected provider after rejected update: %#v", reloaded)
	}
}
