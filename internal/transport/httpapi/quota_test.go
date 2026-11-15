package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
	middleware "promptgate/backend/internal/transport/httpmiddleware"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestHandleCurrentUserQuotaRefreshesChangedAssignmentImmediately(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&users.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	service := subscriptions.NewService(db)
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate subscriptions: %v", err)
	}

	profile := testUserProfile()
	user := users.User{
		ID:                profile.ID,
		ExternalSub:       testPointer(profile.Sub),
		Email:             profile.Email,
		PreferredUsername: profile.PreferredUsername,
		Name:              profile.Name,
		Type:              profile.Type,
		Role:              profile.Role,
		IsActive:          true,
		LastLoginAt:       testPointer(profile.LastLoginAt),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	unlimited, err := service.CreatePlan(ctx, subscriptions.PlanInput{Name: "Unlimited"})
	if err != nil {
		t.Fatalf("create unlimited plan: %v", err)
	}
	quota := int64(100)
	limited, err := service.CreatePlan(ctx, subscriptions.PlanInput{
		Name:          "Limited",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
	})
	if err != nil {
		t.Fatalf("create limited plan: %v", err)
	}
	if err := db.Model(&users.User{}).
		Where("id = ?", user.ID).
		Update("subscription_plan_id", unlimited.ID).Error; err != nil {
		t.Fatalf("assign unlimited plan: %v", err)
	}

	redisServer := miniredis.RunT(t)
	redisBackend, err := redisstore.NewRequired(
		ctx,
		"redis://"+redisServer.Addr(),
		time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("create redis store: %v", err)
	}
	t.Cleanup(func() { _ = redisBackend.Close() })
	quotaStore := subscriptions.NewRedisStore(redisBackend, service, time.Minute, nil)
	quotaStore.SyncVersion(ctx)
	const adminKey = "test-quota-admin-key"
	cfg := config.APIConfig{APIHTTPConfig: config.APIHTTPConfig{AdminAPIKey: adminKey}}
	userService := users.NewService(db)
	handler := NewHandler(Dependencies{Config: cfg, Users: userService, QuotaRedis: quotaStore})
	readAdminQuota := func(id string, wantStatus int) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+id+"/quota", nil)
		request.Header.Set(middleware.AdminAPIKeyHeader, adminKey)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != wantStatus {
			t.Fatalf("admin quota status = %d, want %d: %s", response.Code, wantStatus, response.Body)
		}
		return response
	}

	initial, err := quotaStore.CurrentQuota(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load initial quota: %v", err)
	}
	if initial.Plan == nil || initial.Plan.ID != unlimited.ID || initial.Quota5HTokens != nil {
		t.Fatalf("expected cached unlimited plan, got %#v", initial)
	}
	unlimitedResponse := readAdminQuota(user.ID, http.StatusOK)
	if !strings.Contains(unlimitedResponse.Body.String(), `"quota5hTokens":null`) {
		t.Fatalf("admin quota should be unlimited: %s", unlimitedResponse.Body)
	}

	if err := db.Model(&users.User{}).
		Where("id = ?", user.ID).
		Update("subscription_plan_id", limited.ID).Error; err != nil {
		t.Fatalf("assign limited plan: %v", err)
	}
	redisBackend.Notify(ctx, configevents.DomainSubscriptions)
	if err := quotaStore.IncrementUsage(ctx, user.ID, 24, time.Now()); err != nil {
		t.Fatalf("record quota usage: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/quota", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), profile))
	recorder := httptest.NewRecorder()
	server{quotaRedis: quotaStore}.handleCurrentUserQuota(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	personalQuota := recorder.Body.String()
	var current subscriptions.QuotaStatus
	if err := json.Unmarshal([]byte(personalQuota), &current); err != nil {
		t.Fatalf("decode current quota: %v", err)
	}
	if current.Plan == nil || current.Plan.ID != limited.ID ||
		current.Quota5HTokens == nil || *current.Quota5HTokens != quota ||
		current.Quota7DTokens == nil || *current.Quota7DTokens != quota {
		t.Fatalf("expected limited plan on the first API read, got %#v", current)
	}
	adminQuota := readAdminQuota(user.ID, http.StatusOK)
	if adminQuota.Body.String() != personalQuota || current.Used5HTokens != 24 || current.Used7DTokens != 24 {
		t.Fatalf("admin quota = %s, personal quota = %s", adminQuota.Body, personalQuota)
	}

	withoutPlan, err := userService.CreateUser(ctx, "without-plan")
	if err != nil {
		t.Fatal(err)
	}
	response := readAdminQuota(withoutPlan.ID, http.StatusOK)
	if !strings.Contains(response.Body.String(), `"hasSubscription":false`) {
		t.Fatalf("admin quota should report no subscription: %s", response.Body)
	}
}
