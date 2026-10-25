package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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

func TestHandleCurrentUserQuotaResetUseRequiresIdempotencyKey(t *testing.T) {
	subscriptionService := subscriptions.NewService(nil)
	quotaStore := subscriptions.NewRedisStore(nil, subscriptionService, 0, nil)
	srv := server{subscriptions: subscriptionService, quotaRedis: quotaStore}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/me/quota-resets/use", nil)
	request = request.WithContext(auth.ContextWithUser(request.Context(), auth.UserProfile{ID: "user-id"}))
	recorder := httptest.NewRecorder()

	srv.handleCurrentUserQuotaResetUse(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "{\"error\":\"missing_idempotency_key\"}\n" {
		t.Fatalf("unexpected response %s", recorder.Body.String())
	}
}

func TestWriteQuotaResetErrorUsesBusinessConflictCodes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "unavailable", err: subscriptions.ErrQuotaResetUnavailable, wantStatus: http.StatusConflict, wantBody: "quota_reset_credit_unavailable"},
		{name: "not applicable", err: subscriptions.ErrQuotaResetNotApplicable, wantStatus: http.StatusConflict, wantBody: "quota_reset_not_applicable"},
		{name: "balance full", err: subscriptions.ErrQuotaResetBalanceFull, wantStatus: http.StatusConflict, wantBody: "quota_reset_balance_full"},
		{name: "pending", err: subscriptions.ErrQuotaResetPending, wantStatus: http.StatusConflict, wantBody: "quota_reset_pending"},
		{name: "missing user", err: subscriptions.ErrQuotaResetUserNotFound, wantStatus: http.StatusNotFound, wantBody: "user_not_found"},
		{name: "invalid key", err: subscriptions.ErrInvalidQuotaResetIdempotencyKey, wantStatus: http.StatusBadRequest, wantBody: "missing_idempotency_key"},
		{name: "internal", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantBody: "quota_reset_failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeQuotaResetError(recorder, test.err)
			if recorder.Code != test.wantStatus {
				t.Fatalf("expected %d, got %d", test.wantStatus, recorder.Code)
			}
			if recorder.Body.String() != "{\"error\":\""+test.wantBody+"\"}\n" {
				t.Fatalf("unexpected response %s", recorder.Body.String())
			}
		})
	}
}

func TestCurrentUserQuotaResetHTTPPendingReplayAndAppliedHistory(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	userService := users.NewService(db)
	if err := userService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	subscriptionService := subscriptions.NewService(db)
	if err := subscriptionService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate subscriptions: %v", err)
	}
	profile, err := userService.SyncUser(ctx, auth.Identity{
		Sub: "quota-reset-http", PreferredUsername: "quota-reset-http", Email: "quota-reset@example.com", Name: "Quota Reset HTTP",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	finiteQuota := int64(100)
	if _, err := subscriptionService.CreatePlan(ctx, subscriptions.PlanInput{
		Name: "Quota reset HTTP", Quota5HTokens: &finiteQuota, IsDefault: true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	now := time.Now().UTC()
	if _, err := subscriptionService.GrantQuotaResetCredit(
		ctx,
		profile.ID,
		subscriptions.QuotaResetActor{Type: subscriptions.QuotaResetActorAdminAPIKey},
		"http-grant",
		now,
	); err != nil {
		t.Fatalf("grant credit: %v", err)
	}

	quotaStore := subscriptions.NewRedisStore(nil, subscriptionService, 0, nil)
	srv := server{subscriptions: subscriptionService, quotaRedis: quotaStore}
	useRequest := httptest.NewRequest(http.MethodPost, "/api/v1/me/quota-resets/use", nil)
	useRequest.Header.Set(idempotencyKeyHeader, "http-use")
	useRequest = useRequest.WithContext(auth.ContextWithUser(useRequest.Context(), profile))
	useRecorder := httptest.NewRecorder()
	srv.handleCurrentUserQuotaResetUse(useRecorder, useRequest)
	if useRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", useRecorder.Code, useRecorder.Body.String())
	}
	if strings.Contains(useRecorder.Body.String(), `"quota"`) {
		t.Fatalf("pending reset exposed a non-authoritative live quota: %s", useRecorder.Body.String())
	}
	var useResponse subscriptions.QuotaResetUseResult
	if err := json.NewDecoder(useRecorder.Body).Decode(&useResponse); err != nil {
		t.Fatalf("decode use response: %v", err)
	}
	if useResponse.Status != subscriptions.QuotaResetUsePending || !useResponse.Summary.PendingReset || useResponse.Summary.Available != 0 || useResponse.Quota != nil {
		t.Fatalf("unexpected pending response: %#v", useResponse)
	}

	replayRequest := httptest.NewRequest(http.MethodPost, "/api/v1/me/quota-resets/use", nil)
	replayRequest.Header.Set(idempotencyKeyHeader, "http-use")
	replayRequest = replayRequest.WithContext(auth.ContextWithUser(replayRequest.Context(), profile))
	replayRecorder := httptest.NewRecorder()
	srv.handleCurrentUserQuotaResetUse(replayRecorder, replayRequest)
	if replayRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected idempotent 202 replay, got %d: %s", replayRecorder.Code, replayRecorder.Body.String())
	}

	historyRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me/quota-resets/history?page=1&pageSize=20", nil)
	historyRequest = historyRequest.WithContext(auth.ContextWithUser(historyRequest.Context(), profile))
	historyRecorder := httptest.NewRecorder()
	srv.handleCurrentUserQuotaResetHistory(historyRecorder, historyRequest)
	if historyRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", historyRecorder.Code, historyRecorder.Body.String())
	}
	var history subscriptions.QuotaResetHistoryResult
	if err := json.NewDecoder(historyRecorder.Body).Decode(&history); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if history.Total != 1 || len(history.Items) != 1 || history.Items[0].Type != subscriptions.QuotaResetEventAdminGrant {
		t.Fatalf("pending reset leaked into applied history: %#v", history)
	}
}

func TestCurrentUserQuotaResetHistoryHTTPPaginatesNewestFirst(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	userService := users.NewService(db)
	if err := userService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	subscriptionService := subscriptions.NewService(db)
	if err := subscriptionService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate subscriptions: %v", err)
	}
	profile, err := userService.SyncUser(ctx, auth.Identity{
		Sub: "quota-reset-history", PreferredUsername: "quota-reset-history", Email: "history@example.com", Name: "Quota Reset History",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for index := 0; index < 25; index++ {
		createdAt := base.Add(time.Duration(index) * time.Minute)
		appliedAt := createdAt
		event := subscriptions.QuotaResetEvent{
			UserID:       profile.ID,
			Type:         subscriptions.QuotaResetEventMonthlyGrant,
			Status:       subscriptions.QuotaResetEventApplied,
			Delta:        1,
			BalanceAfter: index % (subscriptions.MaxQuotaResetCredits + 1),
			ActorType:    subscriptions.QuotaResetActorSystem,
			AppliedAt:    &appliedAt,
			CreatedAt:    createdAt,
		}
		if err := db.Create(&event).Error; err != nil {
			t.Fatalf("create history event %d: %v", index, err)
		}
	}

	srv := server{subscriptions: subscriptionService}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/quota-resets/history?page=2&pageSize=10", nil)
	request = request.WithContext(auth.ContextWithUser(request.Context(), profile))
	recorder := httptest.NewRecorder()
	srv.handleCurrentUserQuotaResetHistory(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var history subscriptions.QuotaResetHistoryResult
	if err := json.NewDecoder(recorder.Body).Decode(&history); err != nil {
		t.Fatalf("decode paginated history: %v", err)
	}
	if history.Page != 2 || history.PageSize != 10 || history.Total != 25 || len(history.Items) != 10 {
		t.Fatalf("unexpected pagination metadata: %#v", history)
	}
	if !history.Items[0].CreatedAt.Equal(base.Add(14*time.Minute)) ||
		!history.Items[len(history.Items)-1].CreatedAt.Equal(base.Add(5*time.Minute)) {
		t.Fatalf("history is not ordered newest first on page two: %#v", history.Items)
	}

	ascendingRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/me/quota-resets/history?page=1&pageSize=10&sortBy=createdAt&sortDir=asc",
		nil,
	)
	ascendingRequest = ascendingRequest.WithContext(auth.ContextWithUser(ascendingRequest.Context(), profile))
	ascendingRecorder := httptest.NewRecorder()
	srv.handleCurrentUserQuotaResetHistory(ascendingRecorder, ascendingRequest)
	if ascendingRecorder.Code != http.StatusOK {
		t.Fatalf("expected ascending history 200, got %d: %s", ascendingRecorder.Code, ascendingRecorder.Body.String())
	}
	var ascending subscriptions.QuotaResetHistoryResult
	if err := json.NewDecoder(ascendingRecorder.Body).Decode(&ascending); err != nil {
		t.Fatalf("decode ascending history: %v", err)
	}
	if !ascending.Items[0].CreatedAt.Equal(base) ||
		!ascending.Items[len(ascending.Items)-1].CreatedAt.Equal(base.Add(9*time.Minute)) {
		t.Fatalf("history did not honor ascending date sort: %#v", ascending.Items)
	}

	for _, path := range []string{
		"/api/v1/me/quota-resets/history?sortBy=actor",
		"/api/v1/me/quota-resets/history?sortDir=sideways",
	} {
		invalidRequest := httptest.NewRequest(http.MethodGet, path, nil)
		invalidRequest = invalidRequest.WithContext(auth.ContextWithUser(invalidRequest.Context(), profile))
		invalidRecorder := httptest.NewRecorder()
		srv.handleCurrentUserQuotaResetHistory(invalidRecorder, invalidRequest)
		if invalidRecorder.Code != http.StatusBadRequest || !strings.Contains(invalidRecorder.Body.String(), "invalid_sort") {
			t.Fatalf("expected invalid sort 400 for %q, got %d: %s", path, invalidRecorder.Code, invalidRecorder.Body.String())
		}
	}
}
