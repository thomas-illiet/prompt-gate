package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newQuotaRiskHandlerTest(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := users.NewService(db).AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	service := subscriptions.NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate subscriptions: %v", err)
	}
	return NewHandler(Dependencies{Subscriptions: service}), db
}

// TestHandleAdminDashboardQuotaRiskReturnsSQLProjection verifies the admin
// endpoint is wired to the persisted projection and includes service accounts.
func TestHandleAdminDashboardQuotaRiskReturnsSQLProjection(t *testing.T) {
	handler, db := newQuotaRiskHandlerTest(t)
	now := time.Now().UTC().Add(-time.Minute)
	quota := int64(100)
	account := users.User{
		ID:                "11111111-1111-1111-1111-111111111111",
		ExternalSub:       testPointer("service:quota-risk"),
		Email:             "quota-risk@example.com",
		PreferredUsername: "quota-risk-worker",
		Name:              "Quota risk worker",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       testPointer(now),
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("seed service account: %v", err)
	}
	state := subscriptions.SubscriptionQuotaState{
		UserID:          account.ID,
		HasSubscription: true,
		PlanName:        "Workers",
		Used5HTokens:    80,
		Quota5HTokens:   &quota,
		SyncedAt:        now,
	}
	if err := db.Create(&state).Error; err != nil {
		t.Fatalf("seed quota state: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/quota-risk", nil)
	handler.HandleAdminDashboardQuotaRisk(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response subscriptions.DashboardQuotaRiskResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode quota-risk response: %v", err)
	}
	if response.AtRiskCount != 1 || response.ExhaustedCount != 0 || len(response.Items) != 1 {
		t.Fatalf("unexpected quota-risk response: %#v", response)
	}
	if response.Items[0].AccountType != auth.UserTypeService || response.Items[0].Percent != 80 {
		t.Fatalf("unexpected quota-risk item: %#v", response.Items[0])
	}
}

// TestHandleAdminDashboardQuotaRiskRequiresSubscriptions verifies missing DI is
// reported without panicking.
func TestHandleAdminDashboardQuotaRiskRequiresSubscriptions(t *testing.T) {
	handler := NewHandler(Dependencies{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/quota-risk", nil)

	handler.HandleAdminDashboardQuotaRisk(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "subscriptions service unavailable") {
		t.Fatalf("unexpected error body: %s", recorder.Body.String())
	}
}
