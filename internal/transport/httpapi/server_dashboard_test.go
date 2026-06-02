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

// TestAdminDashboardRoutesRequireAdminRole verifies admin dashboard routes require admin role.
func TestAdminDashboardRoutesRequireAdminRole(t *testing.T) {
	sessionStore := auth.NewSessionStore(nil, time.Hour)
	session, err := sessionStore.CreateSession(testUserProfile(), "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	handler := NewHandler(Dependencies{
		Config:   config.Config{SessionCookieName: "promptgate_session"},
		Sessions: sessionStore,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/tokens", nil)
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
}
