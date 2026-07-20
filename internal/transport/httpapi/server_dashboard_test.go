package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/config"
)

// TestAdminUsageRoutesRequireAdminRole verifies admin usage routes require admin role.
func TestAdminUsageRoutesRequireAdminRole(t *testing.T) {
	sessionStore := auth.NewSessionStore(nil, time.Hour)
	session, err := sessionStore.CreateSession(testUserProfile(), "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	handler := NewHandler(Dependencies{
		Config: config.APIConfig{
			SessionConfig: config.SessionConfig{SessionCookieName: "promptgate_session"},
		},
		Sessions: sessionStore,
	})
	for _, path := range []string{
		"/api/v1/admin/dashboard/tokens",
		"/api/v1/admin/users/11111111-1111-1111-1111-111111111111/statistics",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.AddCookie(&http.Cookie{Name: "promptgate_session", Value: session.ID})
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", recorder.Code, recorder.Body.String())
			}
			var body map[string]string
			if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != "insufficient_role" {
				t.Fatalf("expected insufficient_role, got %#v", body)
			}
		})
	}
}
