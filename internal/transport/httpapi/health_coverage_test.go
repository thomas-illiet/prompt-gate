package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newHealthTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:http-health-%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	return db
}

func TestHandleHealthReportsDatabaseState(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		db := newHealthTestDatabase(t)
		recorder := httptest.NewRecorder()
		(&server{db: db}).handleHealth(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("healthy status: %d %s", recorder.Code, recorder.Body.String())
		}
		var body healthResponse
		if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
			t.Fatalf("decode health response: %v", err)
		}
		if body.Status != "ok" || body.Checks["database"] != "ok" {
			t.Fatalf("unexpected healthy response: %#v", body)
		}
	})

	t.Run("degraded", func(t *testing.T) {
		db := newHealthTestDatabase(t)
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("underlying database: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
		recorder := httptest.NewRecorder()
		(&server{db: db}).handleHealth(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
		if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"status":"degraded"`) || !strings.Contains(recorder.Body.String(), `"database":"error"`) {
			t.Fatalf("degraded status: %d %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestParsePositiveInt(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		fallback int
		want     int
	}{
		{raw: "", fallback: 7, want: 7},
		{raw: "invalid", fallback: 7, want: 7},
		{raw: "0", fallback: 7, want: 7},
		{raw: "-1", fallback: 7, want: 7},
		{raw: "42", fallback: 7, want: 42},
	} {
		if got := parsePositiveInt(tc.raw, tc.fallback); got != tc.want {
			t.Fatalf("parsePositiveInt(%q, %d) = %d, want %d", tc.raw, tc.fallback, got, tc.want)
		}
	}
}
