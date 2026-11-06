package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/notifications"
)

// Public handler contracts must distinguish client mistakes from unavailable
// storage. These requests exercise actual services, rather than error mappers.
func TestFeatureInvalidInputsDoNotMutateData(t *testing.T) {
	h, db := featureHandler(t)
	cases := []struct {
		name             string
		handler          http.HandlerFunc
		body, path, code string
	}{
		{"provider type", h.HandleAdminCreateProvider, `{"name":"provider","type":"unknown"}`, "/providers", "invalid_type"},
		{"provider URL", h.HandleAdminCreateProvider, `{"name":"provider","type":"openai","baseUrl":"file:///tmp/key"}`, "/providers", "invalid_url"},
		{"provider name", h.HandleAdminCreateProvider, `{"name":"","type":"openai"}`, "/providers", "invalid_name"},
		{"provider sorting", h.HandleAdminListProviders, "", "/providers?sortBy=arbitrary", "invalid_sort"},
		{"FAQ question", h.HandleAdminCreateFAQ, `{"question":"","answer":"answer"}`, "/faq", "question_required"},
		{"FAQ answer", h.HandleAdminCreateFAQ, `{"question":"Question?","answer":""}`, "/faq", "answer_required"},
		{"FAQ question limit", h.HandleAdminCreateFAQ, string(mustJSON(t, map[string]any{"question": strings.Repeat("q", 10000), "answer": "answer"})), "/faq", "question_too_long"},
		{"FAQ id", h.HandleAdminGetFAQ, "", "/faq", "invalid_id"},
		{"FAQ sorting", h.HandleAdminListFAQ, "", "/faq?sortDir=arbitrary", "invalid_sort"},
		{"monitor name", h.HandleAdminCreateMonitoringService, `{"name":"","url":"https://example.test"}`, "/monitoring", "invalid_name"},
		{"monitor URL", h.HandleAdminCreateMonitoringService, `{"name":"health","url":"file:///tmp/local"}`, "/monitoring", "invalid_url"},
		{"monitor status", h.HandleAdminCreateMonitoringService, `{"name":"health","url":"https://example.test","expectedStatusCode":999}`, "/monitoring", "invalid_status_code"},
		{"monitor interval", h.HandleAdminCreateMonitoringService, `{"name":"health","url":"https://example.test","expectedStatusCode":200,"intervalSeconds":-1}`, "/monitoring", "invalid_interval"},
		{"monitor sorting", h.HandleAdminListMonitoringServices, "", "/monitoring?sortBy=arbitrary", "invalid_sort"},
		{"user role filter", h.HandleAdminListUsers, "", "/users?role=superuser", "invalid_role"},
		{"user status filter", h.HandleAdminListUsers, "", "/users?status=maybe", "invalid_status"},
		{"user sort", h.HandleAdminListUsers, "", "/users?sortBy=arbitrary", "invalid_sort"},
		{"account identifier", h.HandleAdminCreateServiceAccount, `{"identifier":"bad space","name":"Robot"}`, "/accounts", "invalid_identifier"},
		{"account name", h.HandleAdminCreateServiceAccount, `{"identifier":"robot","name":""}`, "/accounts", "invalid_name"},
		{"account sorting", h.HandleAdminListServiceAccounts, "", "/accounts?sortDir=arbitrary", "invalid_sort"},
		{"mail subject", h.HandleAdminPreviewNotification, `{"subject":"","markdown":"Hello"}`, "/mail", "invalid_email_subject"},
		{"mail body", h.HandleAdminPreviewNotification, `{"subject":"Hello","markdown":""}`, "/mail", "invalid_email_body"},
		{"mail address", h.HandleAdminResolveNotificationAudience, `{"mode":"external","externalAddress":"invalid"}`, "/mail", "invalid_email_address"},
		{"mail audience", h.HandleAdminResolveNotificationAudience, `{"mode":"unknown"}`, "/mail", "invalid_email_audience"},
		{"mail sorting", h.HandleAdminListNotifications, "", "/mail?sortDir=arbitrary", "invalid_sort"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := featureRequest(t, tc.handler, "POST", tc.path, tc.body, 400, "id", "invalid-id")
			if result["error"] != tc.code {
				t.Fatalf("error = %#v, want %s", result, tc.code)
			}
		})
	}
	for _, table := range []string{"providers", "entries", "monitoring_services", "messages"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("invalid input created %d rows in %s", count, table)
		}
	}
}

func TestFeatureMalformedJSONRejectedAcrossMutations(t *testing.T) {
	h, _ := featureHandler(t)
	handlers := map[string]http.HandlerFunc{
		"create provider": h.HandleAdminCreateProvider, "update provider": h.HandleAdminUpdateProvider,
		"create FAQ": h.HandleAdminCreateFAQ, "update FAQ": h.HandleAdminUpdateFAQ, "move FAQ": h.HandleAdminMoveFAQ, "preview FAQ": h.HandleAdminPreviewFAQ,
		"create monitor": h.HandleAdminCreateMonitoringService, "update monitor": h.HandleAdminUpdateMonitoringService,
		"create user": h.HandleAdminCreateUser, "update user": h.HandleAdminUpdateUser, "user note": h.HandleAdminUpdateUserNote,
		"create account": h.HandleAdminCreateServiceAccount, "update account": h.HandleAdminUpdateServiceAccount, "account note": h.HandleAdminUpdateServiceAccountNote,
		"create price": h.HandleAdminCreateModelPrice, "update price": h.HandleAdminUpdateModelPrice, "pricing": h.HandleAdminUpdatePricing, "fallback": h.HandleAdminUpdatePricingFallback,
		"mail preview": h.HandleAdminPreviewNotification, "mail audience": h.HandleAdminResolveNotificationAudience, "mail send": h.HandleAdminSendNotification,
	}
	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			for _, body := range []string{`{`, `{"unexpected":true}`, `{} {}`} {
				featureRequest(t, handler, "POST", "/", body, 400)
			}
		})
	}
}

func TestFeatureStorageFailureIsNotReportedAsSuccess(t *testing.T) {
	h, db := featureHandler(t)
	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		handler http.HandlerFunc
		body    string
	}{
		{"list providers", h.HandleAdminListProviders, ""}, {"get provider", h.HandleAdminGetProvider, ""}, {"delete provider", h.HandleAdminDeleteProvider, ""},
		{"create provider", h.HandleAdminCreateProvider, `{"name":"provider","type":"openai","baseUrl":"https://example.test"}`},
		{"update provider", h.HandleAdminUpdateProvider, `{"displayName":"Changed"}`},
		{"list users", h.HandleAdminListUsers, ""}, {"get user", h.HandleAdminGetUser, ""}, {"delete user", h.HandleAdminDeleteUser, ""},
		{"create user", h.HandleAdminCreateUser, `{"preferredUsername":"human"}`}, {"update user", h.HandleAdminUpdateUser, `{"role":"user","isActive":true}`}, {"user note", h.HandleAdminUpdateUserNote, `{"note":"Changed"}`},
		{"list accounts", h.HandleAdminListServiceAccounts, ""}, {"get account", h.HandleAdminGetServiceAccount, ""}, {"delete account", h.HandleAdminDeleteServiceAccount, ""},
		{"create account", h.HandleAdminCreateServiceAccount, `{"identifier":"robot","name":"Robot"}`}, {"update account", h.HandleAdminUpdateServiceAccount, `{"identifier":"robot","name":"Robot"}`}, {"account note", h.HandleAdminUpdateServiceAccountNote, `{"note":"Changed"}`},
		{"list user tokens", h.HandleAdminListUserTokens, ""}, {"list account tokens", h.HandleAdminListServiceAccountTokens, ""}, {"create account token", h.HandleAdminCreateServiceAccountToken, `{"name":"key"}`}, {"revoke account token", h.HandleAdminRevokeServiceAccountToken, ""},
		{"list FAQ", h.HandleAdminListFAQ, ""}, {"get FAQ", h.HandleAdminGetFAQ, ""}, {"delete FAQ", h.HandleAdminDeleteFAQ, ""}, {"update FAQ", h.HandleAdminUpdateFAQ, `{"question":"Changed?"}`}, {"move FAQ", h.HandleAdminMoveFAQ, `{"position":1}`}, {"create FAQ", h.HandleAdminCreateFAQ, `{"question":"Question?","answer":"Answer"}`},
		{"list monitors", h.HandleAdminListMonitoringServices, ""}, {"get monitor", h.HandleAdminGetMonitoringService, ""}, {"delete monitor", h.HandleAdminDeleteMonitoringService, ""}, {"check monitor", h.HandleAdminCheckMonitoringService, ""},
		{"create monitor", h.HandleAdminCreateMonitoringService, `{"name":"health","url":"https://example.test","expectedStatusCode":200,"intervalSeconds":60}`}, {"update monitor", h.HandleAdminUpdateMonitoringService, `{"enabled":false}`},
		{"pricing", h.HandleAdminGetPricing, ""}, {"get price", h.HandleAdminGetModelPrice, ""}, {"delete price", h.HandleAdminDeleteModelPrice, ""}, {"update price", h.HandleAdminUpdateModelPrice, `{"providerName":"provider","model":"model-a","input":1,"output":2}`},
		{"mail list", h.HandleAdminListNotifications, ""}, {"mail get", h.HandleAdminGetNotification, ""}, {"mail retry", h.HandleAdminRetryNotification, ""},
		{"user IPs", h.HandleAdminListUserIPAddresses, ""}, {"clear user IPs", h.HandleAdminClearUserIPAddresses, ""}, {"account IPs", h.HandleAdminListServiceAccountIPAddresses, ""}, {"clear account IPs", h.HandleAdminClearServiceAccountIPAddresses, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := featureRequest(t, tc.handler, "POST", "/", tc.body, 500, "id", "00000000-0000-4000-8000-000000000001")
			if result["error"] == nil {
				t.Fatal("missing error contract")
			}
		})
	}
}

func TestFeatureNotificationAvailabilityAndIdempotencyContract(t *testing.T) {
	h, db := featureHandler(t)
	for _, key := range []string{"", strings.Repeat("k", 513)} {
		req := httptest.NewRequest("POST", "/mail", strings.NewReader(`{}`))
		req.Header.Set("Idempotency-Key", key)
		rec := httptest.NewRecorder()
		h.HandleAdminSendNotification(rec, req)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_idempotency_key") {
			t.Fatalf("invalid key accepted: %d %s", rec.Code, rec.Body.String())
		}
	}
	for _, handler := range []http.HandlerFunc{h.HandleAdminGetNotification, h.HandleAdminRetryNotification} {
		featureRequest(t, handler, "POST", "/mail", "", 404, "id", "missing")
	}
	h.notifications = notifications.NewService(db, false, "")
	featureRequest(t, h.HandleAdminSendNotification, "POST", "/mail", `{"subject":"Notice","markdown":"Hello","audience":{"mode":"external","externalAddress":"ops@example.test"}}`, 503)
	h.notifications = nil
	for _, handler := range []http.HandlerFunc{h.HandleAdminSendNotification, h.HandleAdminPreviewNotification, h.HandleAdminResolveNotificationAudience, h.HandleAdminListNotifications, h.HandleAdminGetNotification, h.HandleAdminRetryNotification} {
		featureRequest(t, handler, "POST", "/mail", "{}", 503)
	}
}
