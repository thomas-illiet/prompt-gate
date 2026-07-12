package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/faq"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newFAQTestHandler(t *testing.T) (*Handler, *faq.Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service := faq.NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return NewHandler(Dependencies{FAQ: service}), service
}

func TestHandleAdminFAQCRUDAndValidation(t *testing.T) {
	handler, _ := newFAQTestHandler(t)
	create := httptest.NewRequest(http.MethodPost, "/api/v1/admin/faqs", bytes.NewBufferString(`{"question":"How?","answer":"**Safely**","published":true}`))
	createdRecorder := httptest.NewRecorder()
	handler.HandleAdminCreateFAQ(createdRecorder, create)
	if createdRecorder.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", createdRecorder.Code, createdRecorder.Body.String())
	}
	var created faq.Response
	if err := json.NewDecoder(createdRecorder.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(created.RenderedHTML, "<strong>Safely</strong>") {
		t.Fatalf("unexpected render: %s", created.RenderedHTML)
	}

	bad := httptest.NewRequest(http.MethodPost, "/api/v1/admin/faqs", bytes.NewBufferString(`{"question":"","answer":"answer"}`))
	badRecorder := httptest.NewRecorder()
	handler.HandleAdminCreateFAQ(badRecorder, bad)
	if badRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", badRecorder.Code)
	}
}

func TestHandleAdminFAQPreviewSanitizes(t *testing.T) {
	handler, _ := newFAQTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/faqs/preview", bytes.NewBufferString(`{"markdown":"<img src=x onerror=alert(1)> **ok**"}`))
	recorder := httptest.NewRecorder()
	handler.HandleAdminPreviewFAQ(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(body["renderedHtml"]), "onerror") || !strings.Contains(body["renderedHtml"], "<strong>ok</strong>") {
		t.Fatalf("unsafe or invalid preview: %s", body["renderedHtml"])
	}
}
