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

func TestHandleSessionReturnsProfileWithoutFeatureFlags(t *testing.T) {
	sessions := auth.NewSessionStore(nil, time.Hour)
	session, err := sessions.CreateSession(testUserProfile(), "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: "promptgate_session", Value: session.ID})
	recorder := httptest.NewRecorder()
	srv := server{
		config: config.APIConfig{
			SessionConfig: config.SessionConfig{SessionCookieName: "promptgate_session"},
		},
		sessionStore: sessions,
	}

	srv.handleSession(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if response["id"] != session.User.ID {
		t.Fatalf("unexpected session response: %#v", response)
	}
	if _, ok := response["features"]; ok {
		t.Fatalf("always-on features must not be exposed as dynamic flags: %#v", response)
	}
}
