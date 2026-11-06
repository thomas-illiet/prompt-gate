package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/notifications"
)

func TestServiceAccountUpdatePreservesIdentityAndUsage(t *testing.T) {
	service := newTestService(t)
	sqlDB, _ := service.db.DB()
	defer sqlDB.Close()
	ctx := context.Background()
	service.SetNotifier(nil)
	account, err := service.CreateServiceAccount(ctx, ServiceAccountInput{Identifier: "robot", Name: "Robot", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.CreateServiceAccount(ctx, ServiceAccountInput{Identifier: "other", Name: "Other", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	human, err := service.CreateUser(ctx, "human")
	if err != nil {
		t.Fatal(err)
	}
	seedTokenConsumption(t, service.db, account.ID, "request", 10, 20)
	updated, err := service.UpdateServiceAccount(ctx, account.ID, ServiceAccountInput{Identifier: "renamed", Name: "Renamed", IsActive: false})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != account.ID || updated.Identifier != "renamed" || updated.IsActive || updated.TotalTokens != 30 {
		t.Fatalf("identity or historical usage lost: %#v", updated)
	}
	profile, err := service.ServiceAccountProfile(ctx, account.ID)
	if err != nil || profile.Type != auth.UserTypeService || profile.IsActive {
		t.Fatalf("profile stale: %#v %v", profile, err)
	}
	if _, err := service.ServiceAccountProfile(ctx, human.ID); !errors.Is(err, ErrUserNotFound) {
		t.Fatal("human exposed as service account", err)
	}
	if _, err := service.UpdateServiceAccount(ctx, account.ID, ServiceAccountInput{Identifier: other.Identifier, Name: "Collision"}); !errors.Is(err, ErrServiceAccountConflict) {
		t.Fatal("identifier collision accepted", err)
	}
	if _, err := service.UpdateServiceAccount(ctx, human.ID, ServiceAccountInput{Identifier: "takeover", Name: "Takeover"}); !errors.Is(err, ErrUserNotFound) {
		t.Fatal("human account converted", err)
	}
	if _, err := service.UpdateServiceAccount(ctx, account.ID, ServiceAccountInput{}); !errors.Is(err, ErrInvalidServiceAccountIdentifier) {
		t.Fatal(err)
	}
	persisted, err := service.GetServiceAccount(ctx, account.ID)
	if err != nil || persisted.Identifier != "renamed" {
		t.Fatal("rejected update changed persisted identity", err)
	}
	for _, sort := range []string{"totalTokens", "inputTokens", "outputTokens"} {
		list, err := service.ListServiceAccountsPaged(ctx, ServiceAccountListParams{SortBy: sort, SortDir: "desc"})
		if err != nil || list.Items[0].ID != account.ID {
			t.Fatalf("consumption sort %s: %#v %v", sort, list, err)
		}
	}
}

type failingAccessRevoker struct{}

func (failingAccessRevoker) RevokeUserTokensTx(context.Context, *gorm.DB, []string, time.Time) (int64, error) {
	return 0, errors.New("revocation unavailable")
}

func TestAccessExpirationRollsBackIfTokenRevocationFails(t *testing.T) {
	service := newTestService(t)
	sqlDB, _ := service.db.DB()
	defer sqlDB.Close()
	ctx := context.Background()
	service.SetNotificationService(notifications.NewService(service.db, false, ""))
	user, err := service.CreateUser(ctx, "expiring")
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := service.db.Model(&User{}).Where("id = ?", user.ID).Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}
	service.SetTokenRevoker(failingAccessRevoker{})
	if n, err := service.ExpireAccess(ctx, time.Now()); err == nil || n != 0 {
		t.Fatalf("partial expiration committed: %d %v", n, err)
	}
	var persisted User
	if err := service.db.First(&persisted, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Role != auth.RoleLimited || persisted.ExpiresAt == nil {
		t.Fatal("expiration transaction was not rolled back")
	}
	service.SetTokenRevoker(nil)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	service.StartAccessExpiration(runCtx, 10*time.Millisecond)
	deadline := time.Now().Add(time.Second)
	for {
		if err := service.db.First(&persisted, "id = ?", user.ID).Error; err != nil {
			t.Fatal(err)
		}
		if persisted.Role == auth.RoleNone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expiration did not recover")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	if n, err := service.ExpireAccess(ctx, time.Now()); err != nil || n != 0 {
		t.Fatalf("expiration replay changed state: %d %v", n, err)
	}
}
