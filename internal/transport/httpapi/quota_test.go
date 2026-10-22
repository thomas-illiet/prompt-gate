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
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"

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

	initial, err := quotaStore.CurrentQuota(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load initial quota: %v", err)
	}
	if initial.Plan == nil || initial.Plan.ID != unlimited.ID || initial.Quota5HTokens != nil {
		t.Fatalf("expected cached unlimited plan, got %#v", initial)
	}

	if err := db.Model(&users.User{}).
		Where("id = ?", user.ID).
		Update("subscription_plan_id", limited.ID).Error; err != nil {
		t.Fatalf("assign limited plan: %v", err)
	}
	redisBackend.Notify(ctx, configevents.DomainSubscriptions)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/quota", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), profile))
	recorder := httptest.NewRecorder()
	server{quotaRedis: quotaStore}.handleCurrentUserQuota(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var current subscriptions.QuotaStatus
	if err := json.NewDecoder(recorder.Body).Decode(&current); err != nil {
		t.Fatalf("decode current quota: %v", err)
	}
	if current.Plan == nil || current.Plan.ID != limited.ID ||
		current.Quota5HTokens == nil || *current.Quota5HTokens != quota ||
		current.Quota7DTokens == nil || *current.Quota7DTokens != quota {
		t.Fatalf("expected limited plan on the first API read, got %#v", current)
	}
}
