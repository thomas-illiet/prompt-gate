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

	return NewHandler(Dependencies{Providers: service}), service
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

func TestHandleAdminProviderModelCatalog(t *testing.T) {
	handler, service := newProvidersTestHandler(t)
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("expected /models request, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5"},{"id":"gpt-5-mini"}]}`))
	}))
	t.Cleanup(upstream.Close)
	service.SetModelHTTPClient(upstream.Client())

	enabled, err := service.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "openai-main",
		Type:    provider.ProviderTypeOpenAI,
		BaseURL: upstream.URL,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create enabled provider: %v", err)
	}
	disabled, err := service.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "ollama-disabled",
		Type:    provider.ProviderTypeOllama,
		BaseURL: upstream.URL,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("create disabled provider: %v", err)
	}
	disabledState := false
	if _, err := service.UpdateProvider(ctx, disabled.ID.String(), provider.UpdateProviderInput{
		Enabled: &disabledState,
	}); err != nil {
		t.Fatalf("disable provider: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf(
			"/api/v1/admin/providers/model-catalog?providerId=%s&providerId=%s",
			enabled.ID,
			disabled.ID,
		),
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.HandleAdminProviderModelCatalog(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var catalog []provider.ModelCatalogProvider
	if err := json.NewDecoder(recorder.Body).Decode(&catalog); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(catalog) != 2 {
		t.Fatalf("expected two providers, got %#v", catalog)
	}
	if catalog[0].Name != "openai-main" || strings.Join(catalog[0].Models, ",") != "gpt-5,gpt-5-mini" {
		t.Fatalf("unexpected enabled catalog: %#v", catalog[0])
	}
	if catalog[1].Name != "ollama-disabled" || catalog[1].ModelsError != "provider is disabled" {
		t.Fatalf("unexpected disabled catalog: %#v", catalog[1])
	}
}
