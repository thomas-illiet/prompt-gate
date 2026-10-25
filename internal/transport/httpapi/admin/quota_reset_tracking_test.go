package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/subscriptions"
)

func TestHandleAdminListQuotaStatesUsesDefaultsAndCapsPageSize(t *testing.T) {
	handler, db, adminUser, target := newQuotaResetAdminTestHandler(t)
	quota := int64(100)
	now := time.Now().UTC()
	states := []subscriptions.SubscriptionQuotaState{
		{
			UserID: target.ID, HasSubscription: true, PlanName: "Team",
			Used5HTokens: 80, Quota5HTokens: &quota, SyncedAt: now,
		},
		{
			UserID: adminUser.ID, HasSubscription: true, PlanName: "Team",
			Used5HTokens: 20, Quota5HTokens: &quota, SyncedAt: now,
		},
	}
	if err := db.Create(&states).Error; err != nil {
		t.Fatalf("create quota states: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/quota-resets/quota-states?pageSize=500", nil)
	recorder := httptest.NewRecorder()
	handler.HandleAdminListQuotaStates(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var result subscriptions.AdminQuotaStateResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode quota-state response: %v", err)
	}
	if result.Page != 1 || result.PageSize != 100 || result.Total != 1 || len(result.Items) != 1 || result.Items[0].AccountID != target.ID {
		t.Fatalf("unexpected default at-risk response: %+v", result)
	}
}

func TestHandleAdminListQuotaResetEventsUsesDefaultsWithoutIdempotencyKey(t *testing.T) {
	handler, db, _, target := newQuotaResetAdminTestHandler(t)
	now := time.Now().UTC()
	appliedAt := now
	secret := "9c6c40c23a91e2302693933902a1d3ea7e3f7c45bc5146d21ab3867b067c259e"
	event := subscriptions.QuotaResetEvent{
		ID: "40000000-0000-0000-0000-000000000001", UserID: target.ID,
		Type: subscriptions.QuotaResetEventAdminGrant, Status: subscriptions.QuotaResetEventApplied,
		Delta: 1, BalanceAfter: 1, ActorType: subscriptions.QuotaResetActorAdminAPIKey,
		IdempotencyKey: &secret, AppliedAt: &appliedAt, CreatedAt: now,
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatalf("create quota reset event: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/quota-resets/events", nil)
	recorder := httptest.NewRecorder()
	handler.HandleAdminListQuotaResetEvents(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(strings.ToLower(body), "idempotency") || strings.Contains(body, secret) {
		t.Fatalf("response leaked idempotency data: %s", body)
	}
	var result subscriptions.AdminQuotaResetEventResult
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decode quota reset event response: %v", err)
	}
	if result.Window != subscriptions.AdminQuotaResetWindow30D || result.StartsAt == nil || result.Page != 1 || result.PageSize != 20 || result.Total != 1 {
		t.Fatalf("unexpected default event response: %+v", result)
	}
}

func TestAdminQuotaResetTrackingHandlersRejectInvalidEnumsAndSorts(t *testing.T) {
	handler, _, _, _ := newQuotaResetAdminTestHandler(t)
	tests := []struct {
		name     string
		path     string
		handler  func(http.ResponseWriter, *http.Request)
		wantBody string
	}{
		{name: "quota view", path: "/?view=warning", handler: handler.HandleAdminListQuotaStates, wantBody: "invalid_quota_view"},
		{name: "account type", path: "/?accountType=robot", handler: handler.HandleAdminListQuotaStates, wantBody: "invalid_account_type"},
		{name: "quota sort", path: "/?sortBy=quota", handler: handler.HandleAdminListQuotaStates, wantBody: "invalid_sort"},
		{name: "quota sort direction", path: "/?sortDir=sideways", handler: handler.HandleAdminListQuotaStates, wantBody: "invalid_sort"},
		{name: "event window", path: "/?window=90d", handler: handler.HandleAdminListQuotaResetEvents, wantBody: "invalid_usage_window"},
		{name: "event type", path: "/?type=refund", handler: handler.HandleAdminListQuotaResetEvents, wantBody: "invalid_quota_reset_type"},
		{name: "event status", path: "/?status=failed", handler: handler.HandleAdminListQuotaResetEvents, wantBody: "invalid_quota_reset_status"},
		{name: "event sort", path: "/?sortBy=balance", handler: handler.HandleAdminListQuotaResetEvents, wantBody: "invalid_sort"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.handler(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != http.StatusBadRequest || recorder.Body.String() != "{\"error\":\""+test.wantBody+"\"}\n" {
				t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
