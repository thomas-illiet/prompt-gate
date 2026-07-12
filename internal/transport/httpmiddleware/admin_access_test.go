package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
)

const adminSessionCookieName = "promptgate_session"

// TestRequireAdminAccessWithAPIKey verifies API key authentication and its precedence over session authentication.
func TestRequireAdminAccessWithAPIKey(t *testing.T) {
	t.Run("valid key bypasses session without adding a user", func(t *testing.T) {
		handler := newAdminAccessTestHandler(t, "admin-secret", func(w http.ResponseWriter, r *http.Request) {
			if _, ok := auth.UserFromContext(r.Context()); ok {
				t.Fatal("expected API key authentication not to add a user to the context")
			}
			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		req.Header.Set(AdminAPIKeyHeader, "admin-secret")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected success, got status %d", recorder.Code)
		}
	})

	for _, test := range []struct {
		name          string
		configuredKey string
		headerValues  []string
	}{
		{name: "wrong key", configuredKey: "admin-secret", headerValues: []string{"wrong-secret"}},
		{name: "empty key", configuredKey: "admin-secret", headerValues: []string{""}},
		{name: "duplicate key", configuredKey: "admin-secret", headerValues: []string{"admin-secret", "admin-secret"}},
		{name: "key disabled", headerValues: []string{"admin-secret"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := newAdminAccessTestHandler(t, test.configuredKey, func(w http.ResponseWriter, _ *http.Request) {
				t.Fatal("expected request not to reach protected handler")
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
			for _, value := range test.headerValues {
				req.Header.Add(AdminAPIKeyHeader, value)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			assertInvalidAdminAPIKey(t, recorder)
		})
	}

	t.Run("valid key takes precedence over invalid session", func(t *testing.T) {
		handler := newAdminAccessTestHandler(t, "admin-secret", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		req.Header.Set(AdminAPIKeyHeader, "admin-secret")
		req.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: "invalid-session"})
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected API key success, got status %d", recorder.Code)
		}
	})

	t.Run("invalid key does not fall back to valid admin session", func(t *testing.T) {
		store := auth.NewSessionStore(nil, time.Hour)
		session := createAdminAccessTestSession(t, store, auth.RoleAdmin)
		handler := RequireAdminAccess(store, adminSessionCookieName, "admin-secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			t.Fatal("expected request not to reach protected handler")
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		req.Header.Set(AdminAPIKeyHeader, "wrong-secret")
		req.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: session.ID})
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		assertInvalidAdminAPIKey(t, recorder)
	})
}

// TestRequireAdminAccessWithSession verifies that requests without the API key header use the existing admin session chain.
func TestRequireAdminAccessWithSession(t *testing.T) {
	for _, test := range []struct {
		name       string
		role       auth.AppRole
		wantStatus int
	}{
		{name: "admin allowed", role: auth.RoleAdmin, wantStatus: http.StatusNoContent},
		{name: "non admin forbidden", role: auth.RoleManager, wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := auth.NewSessionStore(nil, time.Hour)
			session := createAdminAccessTestSession(t, store, test.role)
			handler := RequireAdminAccess(store, adminSessionCookieName, "admin-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, ok := auth.UserFromContext(r.Context())
				if !ok || user.ID != "user-id" {
					t.Fatal("expected session user in request context")
				}
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
			req.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: session.ID})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d", test.wantStatus, recorder.Code)
			}
		})
	}
}

func newAdminAccessTestHandler(t *testing.T, configuredKey string, next http.HandlerFunc) http.Handler {
	t.Helper()

	store := auth.NewSessionStore(nil, time.Hour)
	return RequireAdminAccess(store, adminSessionCookieName, configuredKey)(next)
}

func createAdminAccessTestSession(t *testing.T, store *auth.SessionStore, role auth.AppRole) auth.Session {
	t.Helper()

	session, err := store.CreateSession(auth.UserProfile{
		ID:       "user-id",
		Role:     role,
		IsActive: true,
	}, "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session
}

func assertInvalidAdminAPIKey(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}
	if body := recorder.Body.String(); body != "{\"error\":\"invalid_admin_api_key\"}\n" {
		t.Fatalf("unexpected response body %q", body)
	}
}
