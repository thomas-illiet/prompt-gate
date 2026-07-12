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

	"promptgate/backend/internal/domain/monitoring"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newMonitoringTestHandler creates monitoring test handler.
func newMonitoringTestHandler(t *testing.T) (*Handler, *monitoring.Service) {
	t.Helper()

	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	service := monitoring.NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate monitoring table: %v", err)
	}

	return NewHandler(Dependencies{Monitoring: service}), service
}

// TestHandleAdminListMonitoringServicesReturnsUnavailableWhenDependencyMissing verifies handle admin list monitoring services returns unavailable when dependency missing.
func TestHandleAdminListMonitoringServicesReturnsUnavailableWhenDependencyMissing(t *testing.T) {
	handler := NewHandler(Dependencies{})
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/monitoring/services?page=1&pageSize=10&sortBy=name&sortDir=asc",
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.HandleAdminListMonitoringServices(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "monitoring service unavailable" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

// TestHandleAdminListMonitoringServicesReturnsPagedServices verifies handle admin list monitoring services returns paged services.
func TestHandleAdminListMonitoringServicesReturnsPagedServices(t *testing.T) {
	handler, service := newMonitoringTestHandler(t)
	_, err := service.CreateService(context.Background(), monitoring.CreateServiceInput{
		Name:               "api",
		URL:                "https://example.com/health",
		ExpectedStatusCode: http.StatusOK,
		IntervalSeconds:    60,
		Enabled:            true,
	})
	if err != nil {
		t.Fatalf("seed monitoring service: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/monitoring/services?page=1&pageSize=10&sortBy=name&sortDir=asc",
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.HandleAdminListMonitoringServices(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body monitoring.ListResult
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 || body.Items[0].Name != "api" {
		t.Fatalf("unexpected list response: %#v", body)
	}
}

// TestHandleAdminCreateMonitoringServiceRejectsInvalidStatus verifies handle admin create monitoring service rejects invalid status.
func TestHandleAdminCreateMonitoringServiceRejectsInvalidStatus(t *testing.T) {
	handler, _ := newMonitoringTestHandler(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/monitoring/services",
		bytes.NewBufferString(`{"name":"api","url":"https://example.com","expectedStatusCode":99,"intervalSeconds":60,"enabled":true}`),
	)
	recorder := httptest.NewRecorder()

	handler.HandleAdminCreateMonitoringService(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "invalid_status_code" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

// TestHandleAdminCheckMonitoringServiceReturnsPersistedResult verifies handle admin check monitoring service returns persisted result.
func TestHandleAdminCheckMonitoringServiceReturnsPersistedResult(t *testing.T) {
	handler, service := newMonitoringTestHandler(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	created, err := service.CreateService(context.Background(), monitoring.CreateServiceInput{
		Name:               "api",
		URL:                upstream.URL,
		ExpectedStatusCode: http.StatusNoContent,
		IntervalSeconds:    60,
		Enabled:            true,
	})
	if err != nil {
		t.Fatalf("seed monitoring service: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/monitoring/services/"+created.ID.String()+"/check",
		nil,
	)
	req.SetPathValue("id", created.ID.String())
	recorder := httptest.NewRecorder()

	handler.HandleAdminCheckMonitoringService(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body monitoring.ServiceResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != monitoring.StatusOK || body.LastStatusCode == nil || *body.LastStatusCode != http.StatusNoContent {
		t.Fatalf("unexpected check response: %#v", body)
	}
}
