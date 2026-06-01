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

func TestNewAPIHandlerWiresMonitoringService(t *testing.T) {
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

	handler := newAPIHandler(&app.App{
		Config: config.Config{
			SessionCookieName: "promptgate_session",
		},
		Monitoring: monitoringService,
		Sessions:   sessionStore,
	})

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
