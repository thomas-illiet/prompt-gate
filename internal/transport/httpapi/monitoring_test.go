package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/monitoring"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newHTTPMonitoringService creates HTTP monitoring service.
func newHTTPMonitoringService(t *testing.T) (*monitoring.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	service := monitoring.NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate monitoring: %v", err)
	}
	return service, db
}

// TestHandleMonitoringStatusReturnsDegradedServicesWithoutURLs verifies handle monitoring status returns degraded services without URLs.
func TestHandleMonitoringStatusReturnsDegradedServicesWithoutURLs(t *testing.T) {
	service, db := newHTTPMonitoringService(t)
	ctx := context.Background()
	created, err := service.CreateService(ctx, monitoring.CreateServiceInput{
		Name:               "api",
		DisplayName:        "API",
		URL:                "https://internal.example.com/health",
		ExpectedStatusCode: http.StatusOK,
		IntervalSeconds:    60,
		Enabled:            true,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	lastCheckedAt := time.Now().UTC()
	lastStatusCode := http.StatusInternalServerError
	if err := db.WithContext(ctx).Model(&monitoring.MonitoringService{}).
		Where("id = ?", created.ID).
		Updates(map[string]any{
			"status":               monitoring.StatusDegraded,
			"last_checked_at":      lastCheckedAt,
			"last_status_code":     lastStatusCode,
			"last_error":           "expected HTTP 200, got 500",
			"consecutive_failures": 1,
		}).Error; err != nil {
		t.Fatalf("mark service degraded: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/status", nil)
	recorder := httptest.NewRecorder()

	server{monitoring: service}.handleMonitoringStatus(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	raw := recorder.Body.String()
	if raw == "" || raw == "null" || json.Valid([]byte(raw)) == false {
		t.Fatalf("expected JSON response, got %q", raw)
	}
	if strings.Contains(raw, "internal.example.com") {
		t.Fatalf("status response leaked URL: %s", raw)
	}

	var body monitoring.StatusResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != monitoring.StatusDegraded || len(body.Services) != 1 {
		t.Fatalf("unexpected monitoring status: %#v", body)
	}
	if body.Services[0].Name != "api" || body.Services[0].DisplayName != "API" {
		t.Fatalf("unexpected degraded service: %#v", body.Services[0])
	}
}
