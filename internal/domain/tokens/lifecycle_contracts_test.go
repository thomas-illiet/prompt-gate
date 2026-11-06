package tokens

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"promptgate/backend/internal/domain/notifications"
)

func TestTokenAdministrationFiltersAndRevocation(t *testing.T) {
	service, userService, db, user := newTokenTestServices(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	ctx := context.Background()
	notificationService := notifications.NewService(db, true, "")
	if err := notificationService.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	service.SetNotificationService(notificationService)
	var created []CreateTokenResponse
	for _, name := range []string{"active", "expired", "revoked"} {
		token, err := service.CreateToken(ctx, user, name, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, token)
	}
	past := time.Now().UTC().Add(-time.Hour)
	if err := db.Model(&Token{}).Where("id = ?", created[1].TokenInfo.ID).Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.markExpired(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := service.AdminRevokeToken(ctx, created[2].TokenInfo.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range []string{"active", "expired", "revoked"} {
		result, err := service.ListTokensPaged(ctx, user.ID, ListParams{Status: status, IncludeRevoked: true, PageSize: 1000, SortBy: "status", SortDir: "asc"})
		if err != nil || result.Total != 1 || len(result.Items) != 1 || result.Items[0].Name != status || result.PageSize != 100 {
			t.Fatalf("filter %s: %#v %v", status, result, err)
		}
	}
	all, err := service.ListTokens(ctx, user.ID)
	if err != nil || len(all) != 3 {
		t.Fatal("complete token history missing", err)
	}
	admin, err := service.AdminListTokens(ctx, user.ID)
	if err != nil || len(admin) != 3 {
		t.Fatal("admin history differs", err)
	}
	visible, err := service.ListTokensFiltered(ctx, user.ID, false)
	if err != nil || len(visible) != 2 {
		t.Fatal("revoked token not filtered", err)
	}
	for _, params := range []ListParams{{Status: "unknown"}, {SortDir: "wrong"}, {SortBy: "DROP TABLE tokens"}} {
		if _, err := service.ListTokensPaged(ctx, user.ID, params); !errors.Is(err, ErrInvalidSort) {
			t.Fatalf("unsafe filter accepted: %v", err)
		}
	}
	if err := service.AdminRevokeToken(ctx, "missing"); !errors.Is(err, ErrTokenNotFound) {
		t.Fatal(err)
	}
	if _, _, err := service.ValidatePrincipalWithExpiry(ctx, created[1].Token, userService); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired key authenticated: %v", err)
	}
	var deliveries int64
	if err := db.Model(&notifications.Message{}).Where("kind IN ?", []notifications.Kind{notifications.KindTokenExpired, notifications.KindTokenRevoked}).Count(&deliveries).Error; err != nil || deliveries != 2 {
		t.Fatalf("duplicate/missing expiry and revocation notifications: %d %v", deliveries, err)
	}
	_ = sqlDB.Close()
	if _, err := service.ListTokens(ctx, user.ID); err == nil {
		t.Fatal("list storage error hidden")
	}
	if _, err := service.AdminListTokens(ctx, user.ID); err == nil {
		t.Fatal("admin list storage error hidden")
	}
	if err := service.AdminRevokeToken(ctx, created[0].TokenInfo.ID); err == nil {
		t.Fatal("revoke storage error hidden")
	}
	if _, _, err := service.ValidatePrincipalWithExpiry(ctx, created[0].Token, userService); err == nil {
		t.Fatal("storage failure authenticated a key")
	}
}

func TestTokenValidationRejectsForgedClaims(t *testing.T) {
	service, userService, db, _ := newTokenTestServices(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	ctx := context.Background()
	for _, tc := range []struct {
		method jwt.SigningMethod
		claims jwt.MapClaims
	}{
		{jwt.SigningMethodHS384, jwt.MapClaims{"sub": "user", "jti": "id", "exp": time.Now().Add(time.Hour).Unix()}},
		{jwt.SigningMethodHS256, jwt.MapClaims{"jti": "id"}},
		{jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user"}},
		{jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user", "jti": "unknown"}},
	} {
		raw, err := jwt.NewWithClaims(tc.method, tc.claims).SignedString([]byte(testJWTSecret))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := service.ValidatePrincipalWithExpiry(ctx, raw, userService); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("forged token accepted: %v", err)
		}
	}
	if _, _, err := service.ValidatePrincipalWithExpiry(ctx, "", userService); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("empty token accepted")
	}
}

func TestBackgroundTokenCleanupPersistsExpiry(t *testing.T) {
	service, _, db, user := newTokenTestServices(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	token, err := service.CreateToken(context.Background(), user, "expired-key", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Token{}).Where("id = ?", token.TokenInfo.ID).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.StartCleanup(ctx, 10*time.Millisecond)
	deadline := time.Now().Add(time.Second)
	for {
		var record Token
		if err := db.First(&record, "id = ?", token.TokenInfo.ID).Error; err != nil {
			t.Fatal(err)
		}
		if record.ExpiredAt != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("token cleanup never ran")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
