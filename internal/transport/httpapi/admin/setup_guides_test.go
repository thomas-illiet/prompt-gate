package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/setupguide"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func newSetupGuidesTestHandler(t *testing.T) (*Handler, *setupguide.Service) {
	t.Helper()
	dsn := fmt.Sprintf("file:admin-setup-guides-%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&setupguide.SetupGuide{}); err != nil {
		t.Fatalf("migrate setup guides: %v", err)
	}
	service := setupguide.NewService(db)
	return NewHandler(Dependencies{SetupGuides: service}), service
}

func setupGuideRequest(method, target, body, id string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if id != "" {
		request.SetPathValue("id", id)
	}
	return request
}

func setupGuideJSON(identifier, title string, position int) string {
	payload, _ := json.Marshal(setupguide.Input{
		Identifier:    identifier,
		Title:         title,
		Subtitle:      "A setup example",
		Icon:          "mdi-code-tags",
		Compatibility: setupguide.CompatibilityBoth,
		ModelMode:     setupguide.ModelModeSingle,
		FilePaths:     []string{" ~/.config/example ", ""},
		Template:      "{{providerDisplayName}}: {{model}} at {{baseUrl}}",
		Enabled:       true,
		Position:      position,
	})
	return string(payload)
}

func createSetupGuideThroughHandler(t *testing.T, handler *Handler, identifier, title string, position int) setupguide.SetupGuide {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.HandleAdminCreateSetupGuide(
		recorder,
		setupGuideRequest(http.MethodPost, "/api/v1/admin/setup-guides", setupGuideJSON(identifier, title, position), ""),
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create setup guide: %d %s", recorder.Code, recorder.Body.String())
	}
	var item setupguide.SetupGuide
	if err := json.NewDecoder(recorder.Body).Decode(&item); err != nil {
		t.Fatalf("decode setup guide: %v", err)
	}
	return item
}

func TestAdminSetupGuideCRUDReorderAndValidation(t *testing.T) {
	handler, _ := newSetupGuidesTestHandler(t)
	first := createSetupGuideThroughHandler(t, handler, "first-guide", "First", 0)
	second := createSetupGuideThroughHandler(t, handler, "second-guide", "Second", 1)
	if first.ID == uuid.Nil || len(first.FilePaths) != 1 || first.FilePaths[0] != "~/.config/example" {
		t.Fatalf("unexpected first guide: %#v", first)
	}

	listRecorder := httptest.NewRecorder()
	handler.HandleAdminListSetupGuides(listRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/setup-guides", nil))
	if listRecorder.Code != http.StatusOK || !strings.Contains(listRecorder.Body.String(), "first-guide") || !strings.Contains(listRecorder.Body.String(), "second-guide") {
		t.Fatalf("list setup guides: %d %s", listRecorder.Code, listRecorder.Body.String())
	}

	getRecorder := httptest.NewRecorder()
	handler.HandleAdminGetSetupGuide(
		getRecorder,
		setupGuideRequest(http.MethodGet, "/api/v1/admin/setup-guides/"+first.ID.String(), "", first.ID.String()),
	)
	if getRecorder.Code != http.StatusOK || !strings.Contains(getRecorder.Body.String(), "first-guide") {
		t.Fatalf("get setup guide: %d %s", getRecorder.Code, getRecorder.Body.String())
	}

	updateRecorder := httptest.NewRecorder()
	handler.HandleAdminUpdateSetupGuide(
		updateRecorder,
		setupGuideRequest(http.MethodPut, "/api/v1/admin/setup-guides/"+first.ID.String(), setupGuideJSON("first-guide", "Updated First", 1), first.ID.String()),
	)
	if updateRecorder.Code != http.StatusOK || !strings.Contains(updateRecorder.Body.String(), "Updated First") {
		t.Fatalf("update setup guide: %d %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	reorderBody, _ := json.Marshal(map[string]any{"ids": []uuid.UUID{second.ID, first.ID}})
	reorderRecorder := httptest.NewRecorder()
	handler.HandleAdminReorderSetupGuides(
		reorderRecorder,
		setupGuideRequest(http.MethodPut, "/api/v1/admin/setup-guides/reorder", string(reorderBody), ""),
	)
	if reorderRecorder.Code != http.StatusOK {
		t.Fatalf("reorder setup guides: %d %s", reorderRecorder.Code, reorderRecorder.Body.String())
	}
	var reordered struct {
		Items []setupguide.SetupGuide `json:"items"`
	}
	if err := json.NewDecoder(reorderRecorder.Body).Decode(&reordered); err != nil {
		t.Fatalf("decode reordered guides: %v", err)
	}
	if len(reordered.Items) != 2 || reordered.Items[0].ID != second.ID || reordered.Items[1].ID != first.ID {
		t.Fatalf("unexpected reordered guides: %#v", reordered.Items)
	}

	for name, tc := range map[string]struct {
		template   string
		wantStatus int
		wantValid  bool
	}{
		"valid":   {template: "{{baseUrl}} {{#models}}{{model}}{{/models}}", wantStatus: http.StatusOK, wantValid: true},
		"invalid": {template: "{{unknown}}", wantStatus: http.StatusBadRequest, wantValid: false},
	} {
		t.Run("validate "+name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"template": tc.template})
			recorder := httptest.NewRecorder()
			handler.HandleAdminValidateSetupGuide(recorder, setupGuideRequest(http.MethodPost, "/validate", string(body), ""))
			if recorder.Code != tc.wantStatus || !strings.Contains(recorder.Body.String(), fmt.Sprintf(`"valid":%t`, tc.wantValid)) {
				t.Fatalf("validate %s: %d %s", name, recorder.Code, recorder.Body.String())
			}
		})
	}

	deleteRecorder := httptest.NewRecorder()
	handler.HandleAdminDeleteSetupGuide(
		deleteRecorder,
		setupGuideRequest(http.MethodDelete, "/api/v1/admin/setup-guides/"+first.ID.String(), "", first.ID.String()),
	)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete setup guide: %d %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	missingRecorder := httptest.NewRecorder()
	handler.HandleAdminGetSetupGuide(
		missingRecorder,
		setupGuideRequest(http.MethodGet, "/api/v1/admin/setup-guides/"+first.ID.String(), "", first.ID.String()),
	)
	if missingRecorder.Code != http.StatusNotFound || !strings.Contains(missingRecorder.Body.String(), "setup_guide_not_found") {
		t.Fatalf("get deleted setup guide: %d %s", missingRecorder.Code, missingRecorder.Body.String())
	}
}

func TestAdminSetupGuideErrorMapping(t *testing.T) {
	handler, _ := newSetupGuidesTestHandler(t)
	existing := createSetupGuideThroughHandler(t, handler, "existing", "Existing", 0)
	missing := uuid.MustParse("00000000-0000-0000-0000-000000000099")

	tests := []struct {
		name       string
		call       func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantError  string
	}{
		{
			name:       "service unavailable",
			call:       NewHandler(Dependencies{}).HandleAdminListSetupGuides,
			request:    httptest.NewRequest(http.MethodGet, "/api/v1/admin/setup-guides", nil),
			wantStatus: http.StatusInternalServerError,
			wantError:  "setup guide service unavailable",
		},
		{
			name:       "invalid id",
			call:       handler.HandleAdminGetSetupGuide,
			request:    setupGuideRequest(http.MethodGet, "/api/v1/admin/setup-guides/not-a-uuid", "", "not-a-uuid"),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid setup guide id",
		},
		{
			name:       "invalid input",
			call:       handler.HandleAdminCreateSetupGuide,
			request:    setupGuideRequest(http.MethodPost, "/api/v1/admin/setup-guides", `{"identifier":"BAD","title":"","icon":"bad","compatibility":"bad","modelMode":"bad","template":""}`, ""),
			wantStatus: http.StatusBadRequest,
			wantError:  "identifier must be lowercase kebab-case",
		},
		{
			name:       "malformed create body",
			call:       handler.HandleAdminCreateSetupGuide,
			request:    setupGuideRequest(http.MethodPost, "/api/v1/admin/setup-guides", `{"identifier":`, ""),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request_body",
		},
		{
			name:       "duplicate identifier",
			call:       handler.HandleAdminCreateSetupGuide,
			request:    setupGuideRequest(http.MethodPost, "/api/v1/admin/setup-guides", setupGuideJSON(existing.Identifier, "Duplicate", 1), ""),
			wantStatus: http.StatusConflict,
			wantError:  "setup_guide_identifier_conflict",
		},
		{
			name:       "missing update",
			call:       handler.HandleAdminUpdateSetupGuide,
			request:    setupGuideRequest(http.MethodPut, "/api/v1/admin/setup-guides/"+missing.String(), setupGuideJSON("missing", "Missing", 1), missing.String()),
			wantStatus: http.StatusNotFound,
			wantError:  "setup_guide_not_found",
		},
		{
			name:       "missing delete",
			call:       handler.HandleAdminDeleteSetupGuide,
			request:    setupGuideRequest(http.MethodDelete, "/api/v1/admin/setup-guides/"+missing.String(), "", missing.String()),
			wantStatus: http.StatusNotFound,
			wantError:  "setup_guide_not_found",
		},
		{
			name:       "incomplete reorder",
			call:       handler.HandleAdminReorderSetupGuides,
			request:    setupGuideRequest(http.MethodPut, "/api/v1/admin/setup-guides/reorder", `{"ids":[]}`, ""),
			wantStatus: http.StatusBadRequest,
			wantError:  "reorder must include every setup guide",
		},
		{
			name:       "malformed validation body",
			call:       handler.HandleAdminValidateSetupGuide,
			request:    setupGuideRequest(http.MethodPost, "/api/v1/admin/setup-guides/validate", `{"template":`, ""),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request_body",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.call(recorder, tc.request)
			if recorder.Code != tc.wantStatus || !strings.Contains(recorder.Body.String(), tc.wantError) {
				t.Fatalf("status/body = %d %s, want %d containing %q", recorder.Code, recorder.Body.String(), tc.wantStatus, tc.wantError)
			}
		})
	}

	if !writeSetupGuideError(httptest.NewRecorder(), context.Canceled) {
		t.Fatal("generic setup guide error was not handled")
	}
}
