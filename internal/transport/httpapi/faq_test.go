package httpapi

import (
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

func TestHandleFAQReturnsPublishedOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service := faq.NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, _ = service.Create(context.Background(), faq.Input{Question: "Visible", Answer: "Answer", Published: true})
	_, _ = service.Create(context.Background(), faq.Input{Question: "Draft", Answer: "Secret", Published: false})
	recorder := httptest.NewRecorder()
	srv := server{faq: service}
	srv.handleFAQ(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/faq", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body []faq.PublicResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body[0].Question != "Visible" {
		t.Fatalf("unexpected body: %#v", body)
	}
}
