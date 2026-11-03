package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	middleware "promptgate/backend/internal/transport/httpmiddleware"
)

func TestRegressionUntrustedOriginMustNotCreateUser(t *testing.T) {
	h, _ := newUsersTestHandler(t)
	store := auth.NewSessionStore(nil, time.Hour)
	session, err := store.CreateSession(auth.UserProfile{ID: "audit-admin", Role: auth.RoleAdmin, IsActive: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware.Chain(http.HandlerFunc(h.HandleAdminCreateUser), middleware.CORS([]string{"https://app.example.com"}), middleware.BrowserRequests([]string{"https://app.example.com"}), middleware.RequireAdminAccess(store, "session", ""))
	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/api/v1/admin/users", strings.NewReader(`{"preferredUsername":"csrf-created-user"}`))
	req.Header.Set("Origin", "https://untrusted.example.com")
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(&http.Cookie{Name: "session", Value: session.ID})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code == http.StatusCreated {
		t.Fatal("untrusted sibling origin with text/plain and session cookie created a user")
	}
}
