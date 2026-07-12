package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/monitoring"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/runtime/app"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestNewAPIHandlerWiresMonitoringService verifies new API handler wires monitoring service.
func TestNewAPIHandlerWiresMonitoringService(t *testing.T) {
	_, sessionStore, handler := newMonitoringAPIHandler(t, config.APIConfig{
		SessionConfig: config.SessionConfig{SessionCookieName: "promptgate_session"},
	})
	session, err := sessionStore.CreateSession(auth.UserProfile{
		ID:                "11111111-1111-1111-1111-111111111111",
		Sub:               "sub",
		PreferredUsername: "admin",
		Email:             "admin@example.com",
		Name:              "Admin",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleAdmin,
		IsActive:          true,
	}, "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/monitoring/services?page=1&pageSize=10&sortBy=name&sortDir=asc",
		nil,
	)
	req.AddCookie(&http.Cookie{Name: "promptgate_session", Value: session.ID})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body monitoring.ListResult
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Total != 0 || len(body.Items) != 0 {
		t.Fatalf("unexpected list response: %#v", body)
	}
}

// TestAdminAPIKeyAuthenticatesAdminRoutes verifies API-key access, precedence, and scope.
func TestAdminAPIKeyAuthenticatesAdminRoutes(t *testing.T) {
	const adminAPIKey = "command-line-admin-key"

	monitoringService, sessionStore, handler := newMonitoringAPIHandler(t, config.APIConfig{
		SessionConfig: config.SessionConfig{SessionCookieName: "promptgate_session"},
		APIHTTPConfig: config.APIHTTPConfig{AdminAPIKey: adminAPIKey},
	})
	adminSession, err := sessionStore.CreateSession(auth.UserProfile{
		ID:                "22222222-2222-2222-2222-222222222222",
		Sub:               "admin-sub",
		PreferredUsername: "admin",
		Email:             "admin@example.com",
		Name:              "Admin",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleAdmin,
		IsActive:          true,
	}, "id-token")
	if err != nil {
		t.Fatalf("create admin session: %v", err)
	}

	t.Run("invalid key does not fall back to admin session", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/admin/monitoring/services",
			strings.NewReader(`{
				"name":"must-not-be-created",
				"url":"https://example.com/health",
				"expectedStatusCode":200,
				"intervalSeconds":60,
				"enabled":true
			}`),
		)
		req.Header.Set("X-Admin-API-Key", "wrong-key")
		req.AddCookie(&http.Cookie{Name: "promptgate_session", Value: adminSession.ID})
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var body map[string]string
		if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
			t.Fatalf("decode unauthorized body: %v", err)
		}
		if body["error"] != "invalid_admin_api_key" {
			t.Fatalf("expected invalid_admin_api_key, got %#v", body)
		}

		result, err := monitoringService.ListServicesPaged(context.Background(), monitoring.ListParams{
			Page: 1, PageSize: 10, SortBy: "name", SortDir: "asc",
		})
		if err != nil {
			t.Fatalf("list monitoring services: %v", err)
		}
		if result.Total != 0 {
			t.Fatalf("expected rejected request not to create a service, got total %d", result.Total)
		}
	})

	t.Run("valid key performs admin mutation without session", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/admin/monitoring/services",
			strings.NewReader(`{
				"name":"api-key-route-test",
				"url":"https://example.com/health",
				"expectedStatusCode":200,
				"intervalSeconds":60,
				"enabled":true
			}`),
		)
		req.Header.Set("X-Admin-API-Key", adminAPIKey)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var created monitoring.ServiceResponse
		if err := json.NewDecoder(recorder.Body).Decode(&created); err != nil {
			t.Fatalf("decode created monitoring service: %v", err)
		}
		if created.Name != "api-key-route-test" {
			t.Fatalf("unexpected created monitoring service: %#v", created)
		}
	})

	t.Run("key is not accepted on non-admin routes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("X-Admin-API-Key", adminAPIKey)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var body map[string]string
		if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
			t.Fatalf("decode unauthorized body: %v", err)
		}
		if body["error"] != "missing session" {
			t.Fatalf("expected existing session error, got %#v", body)
		}
	})
}

// newMonitoringAPIHandler builds an API handler backed by an isolated monitoring database.
func newMonitoringAPIHandler(t *testing.T, cfg config.APIConfig) (*monitoring.Service, *auth.SessionStore, http.Handler) {
	t.Helper()

	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	monitoringService := monitoring.NewService(db)
	if err := monitoringService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate monitoring table: %v", err)
	}

	sessionStore := auth.NewSessionStore(nil, time.Hour)
	return monitoringService, sessionStore, newAPIHandler(&app.App{
		Config:     cfg,
		Monitoring: monitoringService,
		Sessions:   sessionStore,
	})
}
