package pricing

import (
	"context"
	"fmt"
	"strings"
	"testing"

	providerdomain "promptgate/backend/internal/domain/provider"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdateConfigAndRatesForUsesModelThenFallback(t *testing.T) {
	db := newPricingTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}

	_, err := service.UpdateConfig(ctx, UpdateConfigInput{
		Fallback: PriceRates{Input: 1, Output: 2},
		Models: []ModelPriceRecord{{
			ProviderName: "openai-main",
			Model:        "gpt-5",
			Input:        3,
			Output:       4,
		}},
	})
	if err != nil {
		t.Fatalf("update config: %v", err)
	}

	rates, err := service.RatesFor(ctx, "openai-main", "gpt-5")
	if err != nil {
		t.Fatalf("model rates: %v", err)
	}
	if rates.Input != 3 || rates.Output != 4 {
		t.Fatalf("unexpected model rates: %#v", rates)
	}

	fallback, err := service.RatesFor(ctx, "openai-main", "missing")
	if err != nil {
		t.Fatalf("fallback rates: %v", err)
	}
	if fallback.Input != 1 || fallback.Output != 2 {
		t.Fatalf("unexpected fallback rates: %#v", fallback)
	}
}

func TestUpdateConfigRejectsInvalidRows(t *testing.T) {
	db := newPricingTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}

	if _, err := service.UpdateConfig(ctx, UpdateConfigInput{Fallback: PriceRates{Input: -1}}); err != ErrInvalidPrice {
		t.Fatalf("expected invalid price, got %v", err)
	}
	if _, err := service.UpdateConfig(ctx, UpdateConfigInput{
		Fallback: PriceRates{},
		Models:   []ModelPriceRecord{{ProviderName: "openai-main", Model: ""}},
	}); err != ErrInvalidPriceTarget {
		t.Fatalf("expected invalid target, got %v", err)
	}
}

func TestModelPriceCRUDMutatesOnlyTargetRow(t *testing.T) {
	db := newPricingTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}

	first, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "openai-main", Model: "gpt-5", Input: 3, Output: 4})
	if err != nil {
		t.Fatalf("create first model price: %v", err)
	}
	second, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "ollama-local", Model: "llama3", Input: 1, Output: 1})
	if err != nil {
		t.Fatalf("create second model price: %v", err)
	}

	updated, err := service.UpdateModelPrice(ctx, first.ID, ModelPriceRecord{ProviderName: "openai-main", Model: "gpt-5", Input: 5, Output: 6})
	if err != nil {
		t.Fatalf("update first model price: %v", err)
	}
	if updated.Model != "gpt-5" || updated.Input != 5 || updated.Output != 6 {
		t.Fatalf("unexpected updated model price: %#v", updated)
	}

	reloadedSecond, err := service.GetModelPrice(ctx, second.ID)
	if err != nil {
		t.Fatalf("reload second model price: %v", err)
	}
	if reloadedSecond.ProviderName != "ollama-local" || reloadedSecond.Model != "llama3" || reloadedSecond.Input != 1 || reloadedSecond.Output != 1 {
		t.Fatalf("second model price changed unexpectedly: %#v", reloadedSecond)
	}

	if err := service.DeleteModelPrice(ctx, first.ID); err != nil {
		t.Fatalf("delete first model price: %v", err)
	}
	if _, err := service.GetModelPrice(ctx, first.ID); err != ErrPriceNotFound {
		t.Fatalf("expected deleted price to be missing, got %v", err)
	}
	if _, err := service.GetModelPrice(ctx, second.ID); err != nil {
		t.Fatalf("second model price should remain: %v", err)
	}
}

func TestUpdateModelPriceRejectsTargetChanges(t *testing.T) {
	db := newPricingTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}
	created, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "openai-main", Model: "gpt-5", Input: 3, Output: 4})
	if err != nil {
		t.Fatalf("create model price: %v", err)
	}

	if _, err := service.UpdateModelPrice(ctx, created.ID, ModelPriceRecord{ProviderName: "openai-main", Model: "gpt-5.1", Input: 5, Output: 6}); err != ErrImmutablePriceTarget {
		t.Fatalf("expected immutable target error for model change, got %v", err)
	}
	if _, err := service.UpdateModelPrice(ctx, created.ID, ModelPriceRecord{ProviderName: "ollama-local", Model: "gpt-5", Input: 5, Output: 6}); err != ErrImmutablePriceTarget {
		t.Fatalf("expected immutable target error for provider change, got %v", err)
	}

	reloaded, err := service.GetModelPrice(ctx, created.ID)
	if err != nil {
		t.Fatalf("reload model price: %v", err)
	}
	if reloaded.ProviderName != "openai-main" || reloaded.Model != "gpt-5" || reloaded.Input != 3 || reloaded.Output != 4 {
		t.Fatalf("immutable target update changed price unexpectedly: %#v", reloaded)
	}
}

func TestModelPriceCRUDRejectsInvalidAndDuplicateRows(t *testing.T) {
	db := newPricingTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}

	if _, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "", Model: "gpt-5"}); err != ErrInvalidPriceTarget {
		t.Fatalf("expected invalid target, got %v", err)
	}
	if _, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "openai-main", Model: "gpt-5", Input: -1}); err != ErrInvalidPrice {
		t.Fatalf("expected invalid price, got %v", err)
	}
	if _, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "openai-main", Model: "gpt-5"}); err != nil {
		t.Fatalf("create model price: %v", err)
	}
	if _, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "openai-main", Model: "gpt-5"}); err != ErrPriceConflict {
		t.Fatalf("expected price conflict, got %v", err)
	}
}

func TestModelPriceCRUDRejectsUnknownProviderWhenProviderServiceIsConfigured(t *testing.T) {
	db := newPricingTestDB(t)
	providers := providerdomain.NewService(db, nil)
	service := NewService(db, providers)
	ctx := context.Background()
	if err := providers.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate providers: %v", err)
	}
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}
	if _, err := providers.CreateProvider(ctx, providerdomain.CreateProviderInput{
		Name:    "openai-main",
		Type:    providerdomain.ProviderTypeOpenAI,
		BaseURL: "https://api.openai.com/v1",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	if _, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "missing-provider", Model: "gpt-5"}); err != ErrPriceProviderNotFound {
		t.Fatalf("expected missing provider error, got %v", err)
	}
	if _, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "openai-main", Model: "gpt-5"}); err != nil {
		t.Fatalf("expected existing provider price to be accepted, got %v", err)
	}
}

func TestUpdateFallbackDoesNotChangeModelPrices(t *testing.T) {
	db := newPricingTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}
	created, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "openai-main", Model: "gpt-5", Input: 3, Output: 4})
	if err != nil {
		t.Fatalf("create model price: %v", err)
	}

	fallback, err := service.UpdateFallback(ctx, PriceRates{Input: 7, Output: 8})
	if err != nil {
		t.Fatalf("update fallback: %v", err)
	}
	if fallback.Input != 7 || fallback.Output != 8 {
		t.Fatalf("unexpected fallback: %#v", fallback)
	}

	modelPrice, err := service.GetModelPrice(ctx, created.ID)
	if err != nil {
		t.Fatalf("reload model price: %v", err)
	}
	if modelPrice.Input != 3 || modelPrice.Output != 4 {
		t.Fatalf("model price changed unexpectedly: %#v", modelPrice)
	}
}

func newPricingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}
