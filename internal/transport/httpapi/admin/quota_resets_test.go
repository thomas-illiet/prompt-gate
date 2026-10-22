package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestHandleAdminGrantQuotaResetAuditsSessionAndAPIKeyActors(t *testing.T) {
	tests := []struct {
		name          string
		withSession   bool
		wantActorType subscriptions.QuotaResetActorType
		wantActorID   bool
	}{
		{name: "admin session", withSession: true, wantActorType: subscriptions.QuotaResetActorAdminSession, wantActorID: true},
		{name: "admin API key", withSession: false, wantActorType: subscriptions.QuotaResetActorAdminAPIKey, wantActorID: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, db, adminUser, target := newQuotaResetAdminTestHandler(t)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+target.ID+"/quota-resets", nil)
			request.SetPathValue("id", target.ID)
			request.Header.Set("Idempotency-Key", "grant-once")
			if test.withSession {
				request = request.WithContext(auth.ContextWithUser(request.Context(), auth.UserProfile{
					ID: adminUser.ID, Role: auth.RoleAdmin, Type: auth.UserTypeUser, IsActive: true,
				}))
			}
			recorder := httptest.NewRecorder()

			handler.HandleAdminGrantQuotaReset(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
			}
			var event subscriptions.QuotaResetEvent
			if err := db.Where("user_id = ?", target.ID).Take(&event).Error; err != nil {
				t.Fatalf("load quota reset event: %v", err)
			}
			if event.ActorType != test.wantActorType {
				t.Fatalf("expected actor type %q, got %q", test.wantActorType, event.ActorType)
			}
			if test.wantActorID && (event.ActorUserID == nil || *event.ActorUserID != adminUser.ID) {
				t.Fatalf("expected admin actor id %q, got %v", adminUser.ID, event.ActorUserID)
			}
			if !test.wantActorID && event.ActorUserID != nil {
				t.Fatalf("expected nullable API-key actor, got %v", event.ActorUserID)
			}
			if event.IdempotencyKey == nil || *event.IdempotencyKey == "grant-once" || len(*event.IdempotencyKey) != 64 {
				t.Fatalf("expected a hashed idempotency key, got %v", event.IdempotencyKey)
			}
		})
	}
}

func TestHandleAdminGrantQuotaResetRequiresIdempotencyKey(t *testing.T) {
	handler, _, _, target := newQuotaResetAdminTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+target.ID+"/quota-resets", nil)
	request.SetPathValue("id", target.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminGrantQuotaReset(recorder, request)

	if recorder.Code != http.StatusBadRequest || recorder.Body.String() != "{\"error\":\"missing_idempotency_key\"}\n" {
		t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
	}
}

func newQuotaResetAdminTestHandler(t *testing.T) (*Handler, *gorm.DB, auth.UserProfile, auth.UserProfile) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	userService := users.NewService(db)
	if err := userService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	subscriptionService := subscriptions.NewService(db)
	if err := subscriptionService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate subscriptions: %v", err)
	}
	adminUser, err := userService.SyncUser(context.Background(), auth.Identity{
		Sub: "quota-reset-admin", PreferredUsername: "quota-reset-admin", Email: "admin@example.com", Name: "Quota Reset Admin",
	})
	if err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	target, err := userService.SyncUser(context.Background(), auth.Identity{
		Sub: "quota-reset-target", PreferredUsername: "quota-reset-target", Email: "target@example.com", Name: "Quota Reset Target",
	})
	if err != nil {
		t.Fatalf("create target user: %v", err)
	}
	return NewHandler(Dependencies{Users: userService, Subscriptions: subscriptionService}), db, adminUser, target
}
