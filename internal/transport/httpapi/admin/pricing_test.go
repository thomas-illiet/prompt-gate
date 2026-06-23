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

	"promptgate/backend/internal/domain/pricing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPricingTestHandler(t *testing.T) (*Handler, *pricing.Service) {
	t.Helper()

	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	service := pricing.NewService(db, nil)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate pricing table: %v", err)
	}

	return NewHandler(nil, nil, nil, nil, nil, nil, service), service
}

func TestHandleAdminModelPriceCRUD(t *testing.T) {
	handler, service := newPricingTestHandler(t)

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/pricing/models",
		bytes.NewBufferString(`{"providerName":"openai-main","model":"gpt-5","input":3,"output":4}`),
	)
	createRecorder := httptest.NewRecorder()
	handler.HandleAdminCreateModelPrice(createRecorder, createReq)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created pricing.ModelPriceRecord
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}

	updateReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/pricing/models/"+created.ID,
		bytes.NewBufferString(`{"providerName":"openai-main","model":"gpt-5","input":5,"output":6}`),
	)
	updateReq.SetPathValue("id", created.ID)
	updateRecorder := httptest.NewRecorder()
	handler.HandleAdminUpdateModelPrice(updateRecorder, updateReq)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	reloaded, err := service.GetModelPrice(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("reload price: %v", err)
	}
	if reloaded.Model != "gpt-5" || reloaded.Input != 5 || reloaded.Output != 6 {
		t.Fatalf("unexpected updated price: %#v", reloaded)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/pricing/models/"+created.ID, nil)
	deleteReq.SetPathValue("id", created.ID)
	deleteRecorder := httptest.NewRecorder()
	handler.HandleAdminDeleteModelPrice(deleteRecorder, deleteReq)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected delete 204, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
}

func TestHandleAdminUpdateModelPriceRejectsTargetChanges(t *testing.T) {
	handler, _ := newPricingTestHandler(t)

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/pricing/models",
		bytes.NewBufferString(`{"providerName":"openai-main","model":"gpt-5","input":3,"output":4}`),
	)
	createRecorder := httptest.NewRecorder()
	handler.HandleAdminCreateModelPrice(createRecorder, createReq)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created pricing.ModelPriceRecord
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}

	updateReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/pricing/models/"+created.ID,
		bytes.NewBufferString(`{"providerName":"openai-main","model":"gpt-5.1","input":5,"output":6}`),
	)
	updateReq.SetPathValue("id", created.ID)
	updateRecorder := httptest.NewRecorder()
	handler.HandleAdminUpdateModelPrice(updateRecorder, updateReq)
	if updateRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected update 400, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(updateRecorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode immutable body: %v", err)
	}
	if body["error"] != "immutable_price_target" {
		t.Fatalf("unexpected immutable body: %#v", body)
	}
}

func TestHandleAdminUpdatePricingFallback(t *testing.T) {
	handler, service := newPricingTestHandler(t)

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/pricing/fallback",
		bytes.NewBufferString(`{"input":7,"output":8}`),
	)
	recorder := httptest.NewRecorder()
	handler.HandleAdminUpdatePricingFallback(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	rates, err := service.RatesFor(context.Background(), "unknown", "missing")
	if err != nil {
		t.Fatalf("load rates: %v", err)
	}
	if rates.Input != 7 || rates.Output != 8 {
		t.Fatalf("unexpected fallback rates: %#v", rates)
	}
}

func TestHandleAdminModelPriceConflict(t *testing.T) {
	handler, _ := newPricingTestHandler(t)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/admin/pricing/models",
			bytes.NewBufferString(`{"providerName":"openai-main","model":"gpt-5","input":3,"output":4}`),
		)
		recorder := httptest.NewRecorder()
		handler.HandleAdminCreateModelPrice(recorder, req)
		if i == 0 && recorder.Code != http.StatusCreated {
			t.Fatalf("expected first create 201, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if i == 1 {
			if recorder.Code != http.StatusConflict {
				t.Fatalf("expected second create 409, got %d: %s", recorder.Code, recorder.Body.String())
			}
			var body map[string]string
			if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
				t.Fatalf("decode conflict body: %v", err)
			}
			if body["error"] != "pricing_conflict" {
				t.Fatalf("unexpected conflict body: %#v", body)
			}
		}
	}
}
