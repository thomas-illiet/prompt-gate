package proxy

import (
	"context"
	"errors"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
)

func TestProcessInterceptionStartedUpsertsAccountIPAddressesMonotonically(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	worker := NewWorker(db, nil, WorkerOptions{}, nil)
	base := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)

	events := []struct {
		id, accountID, ip string
		seen              time.Time
	}{
		{"a0000000-0000-0000-0000-000000000001", "11111111-1111-1111-1111-111111111111", "198.51.100.8", base},
		{"a0000000-0000-0000-0000-000000000002", "11111111-1111-1111-1111-111111111111", "198.51.100.9", base.Add(time.Minute)},
		{"a0000000-0000-0000-0000-000000000003", "22222222-2222-2222-2222-222222222222", "198.51.100.8", base.Add(2 * time.Minute)},
		{"a0000000-0000-0000-0000-000000000004", "11111111-1111-1111-1111-111111111111", "2001:0db8::1", base.Add(3 * time.Minute)},
		{"a0000000-0000-0000-0000-000000000005", "11111111-1111-1111-1111-111111111111", "198.51.100.8", base.Add(4 * time.Minute)},
		{"a0000000-0000-0000-0000-000000000006", "11111111-1111-1111-1111-111111111111", "198.51.100.8", base.Add(-time.Minute)},
	}
	for index, item := range events {
		event := UsageEvent{EventID: item.id, Type: UsageEventInterceptionStarted, CreatedAt: item.seen,
			InterceptionStarted: &InterceptionStartedEvent{ID: item.id, InitiatorID: item.accountID, Provider: "provider", ProviderType: "openai", Model: "model", ClientIP: item.ip, StartedAt: item.seen, Metadata: "{}"}}
		if err := worker.ProcessUsageEvent(context.Background(), event, item.id+"-redis"); err != nil {
			t.Fatalf("process event %d: %v", index, err)
		}
	}

	result, err := service.ListAccountIPAddresses(context.Background(), events[0].accountID, auth.UserTypeUser, AccountIPAddressListParams{Page: 1, PageSize: 10, SortBy: "lastSeen", SortDir: "desc"})
	if err != nil {
		t.Fatalf("list addresses: %v", err)
	}
	if result.Total != 3 || len(result.Items) != 3 {
		t.Fatalf("expected three distinct addresses, got %#v", result)
	}
	if result.Items[0].IP != "198.51.100.8" || !result.Items[0].LastSeen.Equal(base.Add(4*time.Minute)) {
		t.Fatalf("expected monotonic last seen, got %#v", result.Items[0])
	}
	if result.Items[1].IP != "2001:db8::1" {
		t.Fatalf("expected normalized IPv6, got %#v", result.Items[1])
	}
}

func TestListAccountIPAddressesValidatesAccountTypeAndSort(t *testing.T) {
	_, service := newProxyServiceTestDB(t)
	ctx := context.Background()
	params := AccountIPAddressListParams{Page: 1, PageSize: 10, SortBy: "lastSeen", SortDir: "desc"}
	if _, err := service.ListAccountIPAddresses(ctx, "11111111-1111-1111-1111-111111111111", auth.UserTypeService, params); !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("expected account type mismatch to be not found, got %v", err)
	}
	params.SortBy = "createdAt"
	if _, err := service.ListAccountIPAddresses(ctx, "11111111-1111-1111-1111-111111111111", auth.UserTypeUser, params); !errors.Is(err, ErrInvalidSort) {
		t.Fatalf("expected invalid sort, got %v", err)
	}
}

func TestAccountIPAddressCascadesWithDeletedAccount(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	record := AccountIPAddress{UserID: "11111111-1111-1111-1111-111111111111", IP: "192.0.2.1", LastSeen: time.Now().UTC()}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create address: %v", err)
	}
	if err := db.Delete(&users.User{}, "id = ?", record.UserID).Error; err != nil {
		t.Fatalf("delete account: %v", err)
	}
	var count int64
	if err := db.Model(&AccountIPAddress{}).Where("user_id = ?", record.UserID).Count(&count).Error; err != nil {
		t.Fatalf("count addresses: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected cascade deletion, got %d rows", count)
	}
}
