package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/config"
	httpmiddleware "promptgate/backend/internal/transport/httpmiddleware"
)

func TestOpenAPIRouteUsesCurrentUserSession(t *testing.T) {
	const (
		cookieName = "custom_session"
		adminKey   = "admin-secret"
	)

	sessions := auth.NewSessionStore(nil, time.Hour)
	noRoleSession := createOpenAPITestSession(t, sessions, auth.RoleNone)
	adminSession := createOpenAPITestSession(t, sessions, auth.RoleAdmin)
	userSession := createOpenAPITestSession(t, sessions, auth.RoleUser)
	limitedSession := createOpenAPITestSession(t, sessions, auth.RoleLimited)
	handler := NewHandler(Dependencies{
		Config: config.APIConfig{
			SessionConfig: config.SessionConfig{SessionCookieName: cookieName},
			PublicURLConfig: config.PublicURLConfig{
				BackendBaseURL: "https://api.example.test",
			},
			APIHTTPConfig: config.APIHTTPConfig{AdminAPIKey: adminKey},
		},
		Sessions: sessions,
	})

	for _, test := range []struct {
		name          string
		sessionID     string
		adminKeyValue *string
		wantStatus    int
	}{
		{name: "missing session rejected", wantStatus: http.StatusUnauthorized},
		{name: "role none session rejected", sessionID: noRoleSession.ID, wantStatus: http.StatusForbidden},
		{name: "limited session accepted", sessionID: limitedSession.ID, wantStatus: http.StatusOK},
		{name: "user session accepted", sessionID: userSession.ID, wantStatus: http.StatusOK},
		{name: "admin session accepted", sessionID: adminSession.ID, wantStatus: http.StatusOK},
		{name: "admin API key without user session rejected", adminKeyValue: stringPointer(adminKey), wantStatus: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
			if test.sessionID != "" {
				request.AddCookie(&http.Cookie{Name: cookieName, Value: test.sessionID})
			}
			if test.adminKeyValue != nil {
				request.Header.Set(httpmiddleware.AdminAPIKeyHeader, *test.adminKeyValue)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", test.wantStatus, recorder.Code, recorder.Body.String())
			}
		})
	}

	for _, test := range []struct {
		name          string
		sessionID     string
		adminKeyValue *string
		wantStatus    int
	}{
		{name: "missing admin access rejected", wantStatus: http.StatusUnauthorized},
		{name: "limited session rejected", sessionID: limitedSession.ID, wantStatus: http.StatusForbidden},
		{name: "admin session accepted", sessionID: adminSession.ID, wantStatus: http.StatusOK},
		{name: "admin API key accepted", adminKeyValue: stringPointer(adminKey), wantStatus: http.StatusOK},
	} {
		t.Run("legacy admin alias "+test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openapi.json", nil)
			if test.sessionID != "" {
				request.AddCookie(&http.Cookie{Name: cookieName, Value: test.sessionID})
			}
			if test.adminKeyValue != nil {
				request.Header.Set(httpmiddleware.AdminAPIKeyHeader, *test.adminKeyValue)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", test.wantStatus, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func createOpenAPITestSession(t *testing.T, sessions *auth.SessionStore, role auth.AppRole) auth.Session {
	t.Helper()

	session, err := sessions.CreateSession(auth.UserProfile{
		ID:       string(role) + "-user",
		Role:     role,
		IsActive: true,
	}, "")
	if err != nil {
		t.Fatalf("create %s session: %v", role, err)
	}
	return session
}

func stringPointer(value string) *string {
	return &value
}
