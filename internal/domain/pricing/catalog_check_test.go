package pricing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"promptgate/backend/internal/domain/provider"
)

func TestPricingCheckDistinguishesUnpricedModelsFromProviderOutages(t *testing.T) {
	db := newPricingTestDB(t)
	ctx := context.Background()
	var fail atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"z-model"},{"id":"a-model"},{"id":"priced"}]}`)
	}))
	defer upstream.Close()
	providers := provider.NewService(db, nil)
	if err := providers.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"second", "first"} {
		if _, err := providers.CreateProvider(ctx, provider.CreateProviderInput{Name: name, Type: provider.ProviderTypeOpenAI, BaseURL: upstream.URL, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(db, providers)
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: "first", Model: "priced", Input: 1, Output: 2}); err != nil {
		t.Fatal(err)
	}
	report, err := service.ConfigurationCheck(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Configured || len(report.MissingPrices) != 5 || len(report.ProviderErrors) != 0 || report.MissingPrices[0].ProviderName != "first" || report.MissingPrices[0].Model != "a-model" {
		t.Fatalf("incorrect gap report: %#v", report)
	}
	for _, missing := range report.MissingPrices {
		if _, err := service.CreateModelPrice(ctx, ModelPriceRecord{ProviderName: missing.ProviderName, Model: missing.Model, Input: 1, Output: 2}); err != nil {
			t.Fatal(err)
		}
	}
	report, err = service.ConfigurationCheck(ctx)
	if err != nil || !report.Configured || len(report.MissingPrices) != 0 {
		t.Fatalf("completed prices still reported missing: %#v %v", report, err)
	}
	fail.Store(true)
	report, err = service.ConfigurationCheck(ctx)
	if err != nil || report.Configured || len(report.ProviderErrors) != 2 {
		t.Fatalf("outage mistaken for complete pricing: %#v %v", report, err)
	}
	if _, err := NewService(db, nil).ConfigurationCheck(ctx); err == nil {
		t.Fatal("missing catalog ignored")
	}
	if err := db.Migrator().DropTable(&UsagePrice{}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfigurationCheck(ctx); err == nil {
		t.Fatal("missing pricing storage ignored")
	}
}
