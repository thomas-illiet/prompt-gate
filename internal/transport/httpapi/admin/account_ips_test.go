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
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAccountIPsHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&users.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	if err := proxy.AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy: %v", err)
	}
	for _, account := range []users.User{
		{ID: "11111111-1111-1111-1111-111111111111", ExternalSub: "human", PreferredUsername: "human", Type: auth.UserTypeUser, Role: auth.RoleUser, IsActive: true},
		{ID: "22222222-2222-2222-2222-222222222222", ExternalSub: "service", PreferredUsername: "service", Type: auth.UserTypeService, Role: auth.RoleUser, IsActive: true},
	} {
		if err := db.Create(&account).Error; err != nil {
			t.Fatalf("seed account: %v", err)
		}
	}
	return NewHandler(Dependencies{Users: users.NewService(db), Proxy: proxy.NewService(db)}), db
}

func TestHandleAdminListAccountIPAddressesSeparatesAccountTypes(t *testing.T) {
	handler, db := newAccountIPsHandler(t)
	seen := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	for _, address := range []proxy.AccountIPAddress{
		{UserID: "11111111-1111-1111-1111-111111111111", IP: "192.0.2.10", LastSeen: seen},
		{UserID: "22222222-2222-2222-2222-222222222222", IP: "2001:db8::10", LastSeen: seen.Add(time.Minute)},
	} {
		if err := db.Create(&address).Error; err != nil {
			t.Fatalf("seed address: %v", err)
		}
	}

	tests := []struct {
		name, path, id, wantIP string
		handle                 func(http.ResponseWriter, *http.Request)
	}{
		{"user", "/api/v1/admin/users/id/ips", "11111111-1111-1111-1111-111111111111", "192.0.2.10", handler.HandleAdminListUserIPAddresses},
		{"service", "/api/v1/admin/service-accounts/id/ips", "22222222-2222-2222-2222-222222222222", "2001:db8::10", handler.HandleAdminListServiceAccountIPAddresses},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path+"?page=1&pageSize=10&sortBy=ip&sortDir=asc", nil)
			req.SetPathValue("id", test.id)
			recorder := httptest.NewRecorder()
			test.handle(recorder, req)
			if recorder.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
			}
			var response proxy.AccountIPAddressListResult
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Total != 1 || response.Items[0].IP != test.wantIP {
				t.Fatalf("unexpected response: %#v", response)
			}
		})
	}
}

func TestHandleAdminListAccountIPAddressesMapsErrors(t *testing.T) {
	handler, _ := newAccountIPsHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/id/ips?sortBy=unknown", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	recorder := httptest.NewRecorder()
	handler.HandleAdminListUserIPAddresses(recorder, req)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "invalid_sort") {
		t.Fatalf("expected invalid sort, got %d: %s", recorder.Code, recorder.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/id/ips", nil)
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	recorder = httptest.NewRecorder()
	handler.HandleAdminListUserIPAddresses(recorder, req)
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "user_not_found") {
		t.Fatalf("expected user not found, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
