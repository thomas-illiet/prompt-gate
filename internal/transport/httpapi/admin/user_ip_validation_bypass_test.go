package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
)

func TestAdminUserIPValidationBypassUpdatesAuditsAndFilters(t *testing.T) {
	handler, service, db := newUsersTestHandlerWithDB(t)
	if err := db.AutoMigrate(&users.IPValidationBypassEvent{}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	first, err := service.CreateUser(ctx, "matching-first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateUser(ctx, "matching-second")
	if err != nil {
		t.Fatal(err)
	}
	account, err := service.CreateServiceAccount(ctx, users.ServiceAccountInput{Identifier: "matching-service", Name: "Matching service", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}

	put := func(ctx context.Context, id, body string, wantCode int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body)).WithContext(ctx)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		handler.HandleAdminSetUserIPValidationBypass(rec, req)
		if rec.Code != wantCode {
			t.Fatalf("PUT %s: got %d, want %d: %s", id, rec.Code, wantCode, rec.Body)
		}
		if wantCode == 200 && strings.TrimSpace(rec.Body.String()) != body {
			t.Fatalf("wrong response: %s", rec.Body)
		}
	}

	put(ctx, first.ID, `{"bypassIpValidation":null}`, 400)
	put(ctx, first.ID, `{}`, 400)
	put(ctx, account.ID, `{"bypassIpValidation":true}`, 404)
	actorID := "11111111-1111-4111-8111-111111111111"
	adminCtx := auth.ContextWithUser(ctx, auth.UserProfile{ID: actorID, Role: auth.RoleAdmin})
	put(adminCtx, first.ID, `{"bypassIpValidation":true}`, 200)
	put(adminCtx, first.ID, `{"bypassIpValidation":true}`, 200)

	for _, filter := range []struct {
		value  string
		userID string
	}{
		{"true", first.ID},
		{"false", second.ID},
	} {
		req := httptest.NewRequest(http.MethodGet, "/?pageSize=1&search=matching&role=limited&status=active&bypassIpValidation="+filter.value, nil)
		rec := httptest.NewRecorder()
		handler.HandleAdminListUsers(rec, req)
		var result users.ListResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); rec.Code != 200 || err != nil {
			t.Fatalf("GET filter %s: %d %s (%v)", filter.value, rec.Code, rec.Body, err)
		}
		if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != filter.userID || result.Items[0].BypassIPValidation != (filter.value == "true") {
			t.Fatalf("wrong results for %s: %+v", filter.value, result)
		}
	}
	rec := httptest.NewRecorder()
	handler.HandleAdminListUsers(rec, httptest.NewRequest(http.MethodGet, "/?bypassIpValidation=yes", nil))
	if rec.Code != 400 {
		t.Fatalf("invalid filter returned %d", rec.Code)
	}

	if _, err := service.UpdateUser(ctx, first.ID, users.UpdateUserInput{Role: auth.RoleLimited, IsActive: false}); err != nil {
		t.Fatal(err)
	}
	first, err = service.GetUser(ctx, first.ID)
	if err != nil || !first.BypassIPValidation {
		t.Fatalf("access update reset stored bypass: %+v, %v", first, err)
	}
	put(ctx, first.ID, `{"bypassIpValidation":false}`, 200)

	var events []users.IPValidationBypassEvent
	if err := db.Order("id asc").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].UserID != first.ID || events[0].Previous || !events[0].Enabled ||
		events[0].ActorType != "admin_session" || events[0].ActorUserID == nil || *events[0].ActorUserID != actorID ||
		events[1].UserID != first.ID || !events[1].Previous || events[1].Enabled ||
		events[1].ActorType != "admin_api_key" || events[1].ActorUserID != nil {
		t.Fatalf("wrong audit events: %+v", events)
	}
}
