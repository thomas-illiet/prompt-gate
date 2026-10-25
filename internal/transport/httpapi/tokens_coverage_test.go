package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newUserTokenTestServer(t *testing.T) (server, auth.UserProfile, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:http-user-tokens-%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	userService := users.NewService(db)
	if err := userService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	tokenService := tokens.NewService(db, "coverage-secret")
	if err := tokenService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate tokens: %v", err)
	}
	profile := testUserProfile()
	createHTTPUser(t, db, profile)
	return server{tokenService: tokenService}, profile, db
}

func authenticatedTokenRequest(method, target, body string, profile auth.UserProfile) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	return request.WithContext(auth.ContextWithUser(context.Background(), profile))
}

func TestUserTokenHandlersCRUDAndFiltering(t *testing.T) {
	srv, profile, _ := newUserTokenTestServer(t)
	createRecorder := httptest.NewRecorder()
	srv.handleCreateToken(
		createRecorder,
		authenticatedTokenRequest(http.MethodPost, "/api/v1/me/tokens", `{"name":"coverage_token","description":"covered","expiresInDays":3}`, profile),
	)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create token: %d %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created tokens.CreateTokenResponse
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatalf("decode created token: %v", err)
	}
	if created.Token == "" || created.TokenInfo.UserID != profile.ID || created.TokenInfo.Name != "coverage_token" {
		t.Fatalf("unexpected created token: %#v", created)
	}

	listRecorder := httptest.NewRecorder()
	srv.handleListTokens(
		listRecorder,
		authenticatedTokenRequest(http.MethodGet, "/api/v1/me/tokens?page=1&pageSize=200&search=coverage&status=active&sortBy=name&sortDir=asc", "", profile),
	)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list tokens: %d %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listed tokens.ListResult
	if err := json.NewDecoder(listRecorder.Body).Decode(&listed); err != nil {
		t.Fatalf("decode token list: %v", err)
	}
	if listed.Total != 1 || listed.PageSize != 100 || len(listed.Items) != 1 || listed.Items[0].ID != created.TokenInfo.ID {
		t.Fatalf("unexpected token list: %#v", listed)
	}

	revokeRequest := authenticatedTokenRequest(http.MethodDelete, "/api/v1/me/tokens/"+created.TokenInfo.ID, "", profile)
	revokeRequest.SetPathValue("id", created.TokenInfo.ID)
	revokeRecorder := httptest.NewRecorder()
	srv.handleRevokeToken(revokeRecorder, revokeRequest)
	if revokeRecorder.Code != http.StatusNoContent {
		t.Fatalf("revoke token: %d %s", revokeRecorder.Code, revokeRecorder.Body.String())
	}

	revokedRecorder := httptest.NewRecorder()
	srv.handleListTokens(
		revokedRecorder,
		authenticatedTokenRequest(http.MethodGet, "/api/v1/me/tokens?status=revoked", "", profile),
	)
	if revokedRecorder.Code != http.StatusOK || !strings.Contains(revokedRecorder.Body.String(), created.TokenInfo.ID) {
		t.Fatalf("list revoked tokens: %d %s", revokedRecorder.Code, revokedRecorder.Body.String())
	}

	missingRequest := authenticatedTokenRequest(http.MethodDelete, "/api/v1/me/tokens/missing", "", profile)
	missingRequest.SetPathValue("id", "00000000-0000-0000-0000-000000000099")
	missingRecorder := httptest.NewRecorder()
	srv.handleRevokeToken(missingRecorder, missingRequest)
	if missingRecorder.Code != http.StatusNotFound || !strings.Contains(missingRecorder.Body.String(), "token_not_found") {
		t.Fatalf("revoke missing token: %d %s", missingRecorder.Code, missingRecorder.Body.String())
	}
}

func TestUserTokenHandlersRejectInvalidRequests(t *testing.T) {
	srv, profile, _ := newUserTokenTestServer(t)
	excessiveTTL := 31
	ttlBody, _ := json.Marshal(tokens.CreateTokenRequest{Name: "valid_name", ExpiresInDays: &excessiveTTL})

	tests := []struct {
		name       string
		call       func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantError  string
	}{
		{
			name:       "create without user",
			call:       srv.handleCreateToken,
			request:    httptest.NewRequest(http.MethodPost, "/api/v1/me/tokens", strings.NewReader(`{"name":"valid_name"}`)),
			wantStatus: http.StatusInternalServerError,
			wantError:  "missing user in context",
		},
		{
			name:       "malformed create body",
			call:       srv.handleCreateToken,
			request:    authenticatedTokenRequest(http.MethodPost, "/api/v1/me/tokens", `{"name":`, profile),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request_body",
		},
		{
			name:       "trailing create body",
			call:       srv.handleCreateToken,
			request:    authenticatedTokenRequest(http.MethodPost, "/api/v1/me/tokens", `{"name":"valid_name"} {}`, profile),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request_body",
		},
		{
			name:       "invalid token name",
			call:       srv.handleCreateToken,
			request:    authenticatedTokenRequest(http.MethodPost, "/api/v1/me/tokens", `{"name":"Not Valid"}`, profile),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_token_name",
		},
		{
			name:       "invalid token ttl",
			call:       srv.handleCreateToken,
			request:    authenticatedTokenRequest(http.MethodPost, "/api/v1/me/tokens", string(ttlBody), profile),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_token_ttl",
		},
		{
			name:       "list without user",
			call:       srv.handleListTokens,
			request:    httptest.NewRequest(http.MethodGet, "/api/v1/me/tokens", nil),
			wantStatus: http.StatusInternalServerError,
			wantError:  "missing user in context",
		},
		{
			name:       "invalid list sort",
			call:       srv.handleListTokens,
			request:    authenticatedTokenRequest(http.MethodGet, "/api/v1/me/tokens?sortBy=unknown", "", profile),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_sort",
		},
		{
			name:       "invalid list status",
			call:       srv.handleListTokens,
			request:    authenticatedTokenRequest(http.MethodGet, "/api/v1/me/tokens?status=unknown", "", profile),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_sort",
		},
		{
			name:       "revoke without user",
			call:       srv.handleRevokeToken,
			request:    httptest.NewRequest(http.MethodDelete, "/api/v1/me/tokens/missing", nil),
			wantStatus: http.StatusInternalServerError,
			wantError:  "missing user in context",
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
}

func TestUserTokenHandlersReportStorageErrors(t *testing.T) {
	for name, call := range map[string]func(server, http.ResponseWriter, *http.Request){
		"create": func(srv server, w http.ResponseWriter, r *http.Request) { srv.handleCreateToken(w, r) },
		"list":   func(srv server, w http.ResponseWriter, r *http.Request) { srv.handleListTokens(w, r) },
		"revoke": func(srv server, w http.ResponseWriter, r *http.Request) { srv.handleRevokeToken(w, r) },
	} {
		t.Run(name, func(t *testing.T) {
			srv, profile, db := newUserTokenTestServer(t)
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatalf("underlying database: %v", err)
			}
			if err := sqlDB.Close(); err != nil {
				t.Fatalf("close database: %v", err)
			}
			request := authenticatedTokenRequest(http.MethodGet, "/api/v1/me/tokens", `{"name":"valid_name"}`, profile)
			request.SetPathValue("id", "00000000-0000-0000-0000-000000000099")
			recorder := httptest.NewRecorder()
			call(srv, recorder, request)
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("storage error %s: %d %s", name, recorder.Code, recorder.Body.String())
			}
		})
	}
}
