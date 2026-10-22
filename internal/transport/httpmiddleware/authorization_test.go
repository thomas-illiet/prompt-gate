package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"promptgate/backend/internal/domain/auth"
)

// TestRequireAppAccess verifies app access middleware accepts and rejects the expected roles.
func TestRequireAppAccess(t *testing.T) {
	t.Run("allows active non-none users", func(t *testing.T) {
		handler := RequireAppAccess()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
			Role:     auth.RoleUser,
			IsActive: true,
		}))

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected success, got status %d", recorder.Code)
		}
	})

	t.Run("blocks none role", func(t *testing.T) {
		handler := RequireAppAccess()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
			Role:     auth.RoleNone,
			IsActive: true,
		}))

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusForbidden {
			t.Fatalf("expected forbidden status, got %d", recorder.Code)
		}
	})

	t.Run("blocks legacy manager role", func(t *testing.T) {
		handler := RequireAppAccess()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
			Role:     auth.AppRole("manager"),
			IsActive: true,
		}))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("expected forbidden status, got %d", recorder.Code)
		}
	})
}

// TestRequireRoles verifies role-specific middleware authorization.
func TestRequireRoles(t *testing.T) {
	handler := RequireRoles(auth.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	limitedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	limitedReq = limitedReq.WithContext(auth.ContextWithUser(limitedReq.Context(), auth.UserProfile{
		Role:     auth.RoleLimited,
		IsActive: true,
	}))

	limitedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(limitedRecorder, limitedReq)

	if limitedRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected limited to be forbidden, got %d", limitedRecorder.Code)
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/", nil)
	adminReq = adminReq.WithContext(auth.ContextWithUser(adminReq.Context(), auth.UserProfile{
		Role:     auth.RoleAdmin,
		IsActive: true,
	}))

	adminRecorder := httptest.NewRecorder()
	handler.ServeHTTP(adminRecorder, adminReq)

	if adminRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected admin success, got %d", adminRecorder.Code)
	}
}
