package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"promptgate/backend/internal/domain/faq"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/monitoring"
	"promptgate/backend/internal/domain/notifications"
	"promptgate/backend/internal/domain/pricing"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/setupguide"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/migrations"
	"promptgate/backend/internal/platform/secrets"
)

func featureHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	cipher, err := secrets.NewCipher("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatal(err)
	}
	providers := provider.NewService(db, cipher)
	tokenService := tokens.NewService(db, strings.Repeat("secret", 6))
	userService := users.NewService(db)
	userService.SetTokenRevoker(tokenService)
	return NewHandler(Dependencies{Users: userService, Tokens: tokenService, FAQ: faq.NewService(db), Groups: groups.NewService(db), Providers: providers, Monitoring: monitoring.NewService(db), Notifications: notifications.NewService(db, true, "https://app.test"), Pricing: pricing.NewService(db, providers), Proxy: proxy.NewService(db), Subscriptions: subscriptions.NewService(db), SetupGuides: setupguide.NewService(db)}), db
}

func featureRequest(t *testing.T, handler http.HandlerFunc, method, path, body string, status int, values ...string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "feature-workflow")
	for i := 0; i < len(values); i += 2 {
		req.SetPathValue(values[i], values[i+1])
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != status {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, rec.Code, status, rec.Body.String())
	}
	if status == http.StatusNoContent {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, rec.Body.String())
	}
	return result
}

func TestFeatureProviderAndPricingLifecycle(t *testing.T) {
	h, db := featureHandler(t)
	providerBody := `{"name":"workflow","displayName":"Workflow provider","type":"openai","baseUrl":"https://provider.test/v1","apiKey":"sensitive-fixture-value","enabled":false}`
	created := featureRequest(t, h.HandleAdminCreateProvider, "POST", "/providers", providerBody, 201)
	id := created["id"].(string)
	if strings.Contains(string(mustJSON(t, created)), "sensitive-fixture-value") {
		t.Fatal("provider response exposed secret")
	}
	featureRequest(t, h.HandleAdminCreateProvider, "POST", "/providers", providerBody, 409)
	featureRequest(t, h.HandleAdminListProviders, "GET", "/providers?pageSize=1000", "", 200)
	featureRequest(t, h.HandleAdminGetProvider, "GET", "/providers/"+id, "", 200, "id", id)
	updated := featureRequest(t, h.HandleAdminUpdateProvider, "PATCH", "/providers/"+id, `{"displayName":"Updated","enabled":false}`, 200, "id", id)
	if updated["displayName"] != "Updated" {
		t.Fatal("provider update not returned")
	}
	featureRequest(t, h.HandleAdminGetPricing, "GET", "/pricing", "", 200)
	featureRequest(t, h.HandleAdminUpdatePricingFallback, "PATCH", "/pricing/fallback", `{"input":1.25,"output":4}`, 200)
	price := featureRequest(t, h.HandleAdminCreateModelPrice, "POST", "/prices", `{"providerName":"workflow","model":"model-a","input":0.5,"output":2}`, 201)
	priceID := price["id"].(string)
	featureRequest(t, h.HandleAdminCreateModelPrice, "POST", "/prices", `{"providerName":"workflow","model":"model-a","input":0.5,"output":2}`, 409)
	featureRequest(t, h.HandleAdminGetModelPrice, "GET", "/prices/"+priceID, "", 200, "id", priceID)
	featureRequest(t, h.HandleAdminUpdateModelPrice, "PATCH", "/prices/"+priceID, `{"providerName":"workflow","model":"model-a","input":0.75,"output":3}`, 200, "id", priceID)
	featureRequest(t, h.HandleAdminUpdateModelPrice, "PATCH", "/prices/"+priceID, `{"providerName":"workflow","model":"different","input":1,"output":2}`, 400, "id", priceID)
	featureRequest(t, h.HandleAdminPricingConfigurationCheck, "GET", "/pricing/check", "", 200)
	featureRequest(t, h.HandleAdminDeleteModelPrice, "DELETE", "/prices/"+priceID, "", 204, "id", priceID)
	featureRequest(t, h.HandleAdminGetModelPrice, "GET", "/prices/"+priceID, "", 404, "id", priceID)
	featureRequest(t, h.HandleAdminUpdatePricing, "PUT", "/pricing", `{"fallback":{"input":2,"output":6},"models":[{"providerName":"workflow","model":"model-b","input":0.2,"output":1}]}`, 200)
	var rows int64
	if err := db.Model(&pricing.UsagePrice{}).Where("scope = ?", pricing.ScopeModel).Count(&rows).Error; err != nil || rows != 1 {
		t.Fatalf("price replacement was not atomic: %d %v", rows, err)
	}
	featureRequest(t, h.HandleAdminDeleteProvider, "DELETE", "/providers/"+id, "", 204, "id", id)
	featureRequest(t, h.HandleAdminGetProvider, "GET", "/providers/"+id, "", 404, "id", id)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestFeatureFAQPublicationAndReordering(t *testing.T) {
	h, _ := featureHandler(t)
	first := featureRequest(t, h.HandleAdminCreateFAQ, "POST", "/faqs", `{"question":"First?","answer":"**Safe**","published":true}`, 201)
	id := first["id"].(string)
	featureRequest(t, h.HandleAdminCreateFAQ, "POST", "/faqs", `{"question":"Second?","answer":"Draft","published":false}`, 201)
	featureRequest(t, h.HandleAdminListFAQ, "GET", "/faqs?pageSize=1000", "", 200)
	featureRequest(t, h.HandleAdminUpdateFAQ, "PATCH", "/faqs/"+id, `{"question":"Edited?","answer":"Updated","published":false}`, 200, "id", id)
	item := featureRequest(t, h.HandleAdminGetFAQ, "GET", "/faqs/"+id, "", 200, "id", id)
	if item["question"] != "Edited?" || item["published"] != false {
		t.Fatalf("publication state: %#v", item)
	}
	featureRequest(t, h.HandleAdminMoveFAQ, "PATCH", "/faqs/position", `{"position":1}`, 200, "id", id)
	featureRequest(t, h.HandleAdminDeleteFAQ, "DELETE", "/faqs/"+id, "", 204, "id", id)
	featureRequest(t, h.HandleAdminGetFAQ, "GET", "/faqs/"+id, "", 404, "id", id)
}

func TestFeatureMonitoringDetectsUpstreamFailure(t *testing.T) {
	h, _ := featureHandler(t)
	var upstreamStatus atomic.Int32
	upstreamStatus.Store(200)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(int(upstreamStatus.Load())) }))
	defer upstream.Close()
	body := string(mustJSON(t, map[string]any{"name": "health", "url": upstream.URL, "expectedStatusCode": 200, "intervalSeconds": 60, "enabled": true}))
	created := featureRequest(t, h.HandleAdminCreateMonitoringService, "POST", "/monitoring", body, 201)
	id := created["id"].(string)
	featureRequest(t, h.HandleAdminCreateMonitoringService, "POST", "/monitoring", body, 409)
	featureRequest(t, h.HandleAdminGetMonitoringService, "GET", "/monitoring/"+id, "", 200, "id", id)
	featureRequest(t, h.HandleAdminListMonitoringServices, "GET", "/monitoring", "", 200)
	featureRequest(t, h.HandleAdminCheckMonitoringService, "POST", "/monitoring/check", "", 200, "id", id)
	upstreamStatus.Store(503)
	featureRequest(t, h.HandleAdminCheckMonitoringService, "POST", "/monitoring/check", "", 200, "id", id)
	service, err := h.monitoring.GetService(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if service.LastStatusCode == nil || *service.LastStatusCode != 503 {
		t.Fatalf("outage not retained: %#v", service)
	}
	featureRequest(t, h.HandleAdminUpdateMonitoringService, "PATCH", "/monitoring/"+id, `{"enabled":false}`, 200, "id", id)
	featureRequest(t, h.HandleAdminDeleteMonitoringService, "DELETE", "/monitoring/"+id, "", 204, "id", id)
	featureRequest(t, h.HandleAdminGetMonitoringService, "GET", "/monitoring/"+id, "", 404, "id", id)
}

func TestFeatureNotificationPreviewSendHistoryAndRetry(t *testing.T) {
	h, db := featureHandler(t)
	preview := featureRequest(t, h.HandleAdminPreviewNotification, "POST", "/notifications/preview", `{"subject":"Maintenance","markdown":"**Hello** <script>owned()</script>"}`, 200)
	if strings.Contains(preview["htmlBody"].(string), "<script") {
		t.Fatal("unsafe notification preview")
	}
	audience := featureRequest(t, h.HandleAdminResolveNotificationAudience, "POST", "/notifications/audience", `{"mode":"external","externalAddress":"ops@example.test"}`, 200)
	if audience["valid"] != float64(1) {
		t.Fatalf("audience not resolved: %#v", audience)
	}
	body := `{"subject":"Maintenance","markdown":"Scheduled maintenance","audience":{"mode":"external","externalAddress":"ops@example.test"}}`
	result := featureRequest(t, h.HandleAdminSendNotification, "POST", "/notifications", body, 202)
	id := result["message"].(map[string]any)["id"].(string)
	replay := featureRequest(t, h.HandleAdminSendNotification, "POST", "/notifications", body, 202)
	if replay["message"].(map[string]any)["id"] != id {
		t.Fatal("idempotent send duplicated mail")
	}
	featureRequest(t, h.HandleAdminListNotifications, "GET", "/notifications?search=Maintenance&kind=manual", "", 200)
	featureRequest(t, h.HandleAdminGetNotification, "GET", "/notifications/"+id, "", 200, "id", id)
	if err := db.Model(&notifications.Delivery{}).Where("message_id = ?", id).Update("status", notifications.DeliveryFailed).Error; err != nil {
		t.Fatal(err)
	}
	retried := featureRequest(t, h.HandleAdminRetryNotification, "POST", "/notifications/retry", "", 200, "id", id)
	if retried["retried"] != float64(1) {
		t.Fatalf("failed delivery not retried: %#v", retried)
	}
	featureRequest(t, h.HandleAdminRetryNotification, "POST", "/notifications/retry", "", 200, "id", id)
	var delivery notifications.Delivery
	if err := db.Where("message_id = ?", id).First(&delivery).Error; err != nil || delivery.Status != notifications.DeliveryPending {
		t.Fatalf("retry persisted state: %#v %v", delivery, err)
	}
}

func TestFeatureHumanAndServiceAccountSeparation(t *testing.T) {
	h, _ := featureHandler(t)
	user := featureRequest(t, h.HandleAdminCreateUser, "POST", "/users", `{"preferredUsername":"human"}`, 201)
	userID := user["id"].(string)
	account := featureRequest(t, h.HandleAdminCreateServiceAccount, "POST", "/accounts", `{"identifier":"robot","name":"Robot","isActive":true}`, 201)
	accountID := account["id"].(string)
	featureRequest(t, h.HandleAdminListUsers, "GET", "/users?role=limited&status=active&pageSize=1000", "", 200)
	featureRequest(t, h.HandleAdminGetUser, "GET", "/users/"+userID, "", 200, "id", userID)
	featureRequest(t, h.HandleAdminListServiceAccounts, "GET", "/accounts?pageSize=1000", "", 200)
	featureRequest(t, h.HandleAdminGetServiceAccount, "GET", "/accounts/"+accountID, "", 200, "id", accountID)
	featureRequest(t, h.HandleAdminGetServiceAccount, "GET", "/accounts/"+userID, "", 404, "id", userID)
	featureRequest(t, h.HandleAdminCreateUserToken, "POST", "/users/"+accountID, `{"name":"wrong owner","expiresInDays":7}`, 404, "id", accountID)
	featureRequest(t, h.HandleAdminUpdateUser, "PATCH", "/users/"+userID, `{"role":"user","isActive":true}`, 200, "id", userID)
	featureRequest(t, h.HandleAdminUpdateUserNote, "PATCH", "/users/note", `{"note":"Reviewed"}`, 200, "id", userID)
	featureRequest(t, h.HandleAdminUpdateServiceAccount, "PATCH", "/accounts/"+accountID, `{"identifier":"robot","name":"Updated robot","isActive":true}`, 200, "id", accountID)
	featureRequest(t, h.HandleAdminUpdateServiceAccountNote, "PATCH", "/accounts/note", `{"note":"Automation"}`, 200, "id", accountID)
	humanToken := featureRequest(t, h.HandleAdminCreateUserToken, "POST", "/users/token", `{"name":"human-token","expiresInDays":7}`, 201, "id", userID)
	robotToken := featureRequest(t, h.HandleAdminCreateServiceAccountToken, "POST", "/accounts/token", `{"name":"robot-token","expiresInDays":7}`, 201, "id", accountID)
	featureRequest(t, h.HandleAdminListUserTokens, "GET", "/users/tokens", "", 200, "id", userID)
	featureRequest(t, h.HandleAdminListServiceAccountTokens, "GET", "/accounts/tokens?includeRevoked=true", "", 200, "id", accountID)
	// Disabling an account revokes its existing credentials, not merely its UI access.
	featureRequest(t, h.HandleAdminUpdateUser, "PATCH", "/users/"+userID, `{"role":"none","isActive":false}`, 200, "id", userID)
	if _, _, err := h.tokens.ValidatePrincipalWithExpiry(context.Background(), humanToken["token"].(string), h.users); err == nil {
		t.Fatal("disabled user's token remained usable")
	}
	featureRequest(t, h.HandleAdminRevokeServiceAccountToken, "DELETE", "/accounts/token", "", 204, "id", accountID, "tokenId", robotToken["tokenInfo"].(map[string]any)["id"].(string))
	featureRequest(t, h.HandleAdminDeleteServiceAccount, "DELETE", "/accounts/"+accountID, "", 204, "id", accountID)
	featureRequest(t, h.HandleAdminDeleteUser, "DELETE", "/users/"+userID, "", 204, "id", userID)
}
