package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/config"
)

func TestHandleSessionReturnsProfileAndAdminContactEmail(t *testing.T) {
	tests := []struct {
		name              string
		adminContactEmail string
	}{
		{name: "configured", adminContactEmail: "admin@example.com"},
		{name: "empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
					APIHTTPConfig: config.APIHTTPConfig{AdminContactEmail: test.adminContactEmail},
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
			if response["id"] != session.User.ID || response["adminContactEmail"] != test.adminContactEmail {
				t.Fatalf("unexpected session response: %#v", response)
			}
			if _, ok := response["features"]; ok {
				t.Fatalf("always-on features must not be exposed as dynamic flags: %#v", response)
			}
		})
	}
}

func TestHandleSessionDoesNotExposeAdminContactWithoutSession(t *testing.T) {
	recorder := httptest.NewRecorder()
	srv := server{
		config: config.APIConfig{
			SessionConfig: config.SessionConfig{SessionCookieName: "promptgate_session"},
			APIHTTPConfig: config.APIHTTPConfig{AdminContactEmail: "admin@example.com"},
		},
		sessionStore: auth.NewSessionStore(nil, time.Hour),
	}

	srv.handleSession(recorder, httptest.NewRequest(http.MethodGet, "/auth/session", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "admin@example.com") {
		t.Fatalf("unauthenticated response exposed administrator contact: %s", recorder.Body.String())
	}
}
