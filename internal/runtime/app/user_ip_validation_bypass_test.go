package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/policy"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/clientip"
)

type bypassTestAuthCache struct {
	tokens.NoopAuthCache
	principals map[string]auth.Principal
}

func (c *bypassTestAuthCache) Get(_ context.Context, hash string) (auth.Principal, int64, bool, error) {
	principal, ok := c.principals[hash]
	return principal, 0, ok, nil
}

func (c *bypassTestAuthCache) Set(_ context.Context, _ int64, hash string, principal auth.Principal, _ time.Duration) {
	c.principals[hash] = principal
}

type failingBypassResolver struct{}

func (failingBypassResolver) CanBypassIPValidation(context.Context, string) (bool, error) {
	return true, errors.New("database unavailable")
}

func TestUserIPValidationBypassUsesCurrentAccountWithCachedKey(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	accounts := users.NewService(db)
	tokenService := tokens.NewService(db, "0123456789abcdef0123456789abcdef")
	if err := db.AutoMigrate(&users.User{}, &users.IPValidationBypassEvent{}, &tokens.Token{}); err != nil {
		t.Fatal(err)
	}
	user, err := accounts.SyncUser(ctx, auth.Identity{Sub: "bypass-user", PreferredUsername: "bypass-user"})
	if err != nil {
		t.Fatal(err)
	}
	days := 1
	key, err := tokenService.CreateToken(ctx, user, "bypass-test", "", &days)
	if err != nil {
		t.Fatal(err)
	}

	validationCalls := 0
	client, err := policy.NewClient("https://validation.example/ips/{ip}", "secret", &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		validationCalls++
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	access := groups.NewSnapshotStore(nil)
	if err := access.SetSnapshot(groups.Snapshot{
		KnownProviders: []string{"openai-test"},
		ProviderTypes:  map[string]provider.ProviderType{"openai-test": provider.ProviderTypeOpenAI},
		Users: map[string]groups.UserAccess{user.ID: {
			Rules: []groups.AccessRule{{Providers: []string{"openai-test"}, ModelPatterns: []string{"^gpt-test$"}}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	cache := &bypassTestAuthCache{principals: make(map[string]auth.Principal)}
	build := func(resolver policy.UserBypassResolver) http.Handler {
		evaluator := policy.NewEvaluator(client, nil, time.Minute, nil)
		downstream := groups.Middleware(access, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		checked := policy.Middleware(evaluator, nil, resolver, nil)(downstream)
		return tokens.MiddlewareWithOptions(tokens.MiddlewareOptions{
			TokenService: tokenService, UserResolver: accounts, Cache: cache,
		})(clientip.Middleware(false)(checked))
	}
	first, second := build(accounts), build(users.NewService(db))
	request := func(handler http.Handler, token, model string, wantCode int, wantBody string, wantCalls int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/openai-test/v1/chat/completions", strings.NewReader(`{"model":"`+model+`"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != wantCode || !strings.Contains(rec.Body.String(), wantBody) || validationCalls != wantCalls {
			t.Fatalf("status=%d want=%d body=%s validation calls=%d want=%d", rec.Code, wantCode, rec.Body, validationCalls, wantCalls)
		}
	}

	request(first, key.Token, "gpt-test", 403, "ip_not_allowed", 1)
	if len(cache.principals) != 1 {
		t.Fatal("authentication cache was not primed")
	}
	if err := accounts.SetIPValidationBypass(ctx, user.ID, true, nil); err != nil {
		t.Fatal(err)
	}
	request(second, key.Token, "gpt-test", 204, "", 1)
	request(first, key.Token, "unavailable-model", 403, "group_access_denied", 1)
	request(second, "invalid-key", "gpt-test", 401, "invalid_token", 1)
	request(build(failingBypassResolver{}), key.Token, "gpt-test", 403, "ip_not_allowed", 2)

	if err := accounts.SetIPValidationBypass(ctx, user.ID, false, nil); err != nil {
		t.Fatal(err)
	}
	request(second, key.Token, "gpt-test", 403, "ip_not_allowed", 3)
}
