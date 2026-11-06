package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	cdrslog "cdr.dev/slog/v3"
	"github.com/alicebob/miniredis/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/provider"
	usage "promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/migrations"
	"promptgate/backend/internal/platform/testconfig"
)

// Every runtime gets an isolated PostgreSQL schema and Redis instance. A separate
// observer connection lets us detect leaked pools even when construction fails.
func lifecycleConfig(t *testing.T) (config.APIConfig, *gorm.DB, string) {
	t.Helper()
	cfg, err := config.LoadApi(testconfig.Path(t))
	if err != nil {
		t.Fatal(err)
	}
	observer, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		testconfig.Unavailable(t, "PostgreSQL", err)
	}
	sqlDB, _ := observer.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	schema := fmt.Sprintf("runtime_%d", time.Now().UnixNano())
	if err := observer.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := observer.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	endpoint, _ := url.Parse(cfg.DatabaseURL)
	q := endpoint.Query()
	q.Set("search_path", schema)
	endpoint.RawQuery = q.Encode()
	fixture, err := gorm.Open(postgres.Open(endpoint.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	fixtureSQL, _ := fixture.DB()
	defer fixtureSQL.Close()
	if err := migrations.Run(context.Background(), fixture); err != nil {
		t.Fatal(err)
	}
	q.Set("application_name", schema)
	endpoint.RawQuery = q.Encode()
	cfg.DatabaseURL = endpoint.String()
	redis := miniredis.RunT(t)
	cfg.RedisURL = "redis://" + redis.Addr()
	cfg.KeycloakIssuerURL = ""
	cfg.KeycloakJWKSURL = ""
	return cfg, observer, schema
}

func TestScheduleRuntimeExpiresAccessAndTokens(t *testing.T) {
	api, observer, name := lifecycleConfig(t)
	cfg, err := config.LoadSchedule(testconfig.Path(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.DatabaseURL = api.DatabaseURL
	cfg.RedisURL = api.RedisURL
	cfg.NotificationsEnabled = true
	cfg.TokenCleanupInterval = 20 * time.Millisecond
	cfg.UserAccessExpirationInterval = 20 * time.Millisecond
	cfg.SubscriptionQuotaSyncInterval = 20 * time.Millisecond
	cfg.QuotaResetDistributionInterval = 20 * time.Millisecond
	cfg.NotificationCleanupInterval = 20 * time.Millisecond
	cfg.QuotaNotificationInterval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := NewSchedule(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.ModelLab != nil || a.OIDC != nil {
		t.Fatal("scheduler unexpectedly initialized API-only services")
	}
	user, err := a.Users.CreateUser(ctx, "expiring")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := a.Users.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	key, err := a.Tokens.CreateToken(ctx, profile, "expiring-key", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Hour)
	if err := a.DB.Model(&tokens.Token{}).Where("id = ?", key.TokenInfo.ID).Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}
	// Use another account so access revocation cannot hide token-expiry cleanup.
	account, err := a.Users.CreateUser(ctx, "access-expiring")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Model(&users.User{}).Where("id = ?", account.ID).Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}
	a.StartBackgroundJobs(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for {
		var record tokens.Token
		if err := a.DB.First(&record, "id = ?", key.TokenInfo.ID).Error; err != nil {
			t.Fatal(err)
		}
		current, err := a.Users.UserByID(ctx, account.ID)
		if err != nil {
			t.Fatal(err)
		}
		if record.ExpiredAt != nil && current.Role == auth.RoleNone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("scheduled expiration did not run: token=%#v user=%#v", record.ExpiredAt, current)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	assertRuntimeConnectionsClosed(t, observer, name)
}

func assertRuntimeConnectionsClosed(t *testing.T, observer *gorm.DB, name string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		var count int64
		if err := observer.Raw("SELECT count(*) FROM pg_stat_activity WHERE application_name = ?", name).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("runtime leaked %d PostgreSQL connections", count)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRuntimeInitializationFailureReleasesConnections(t *testing.T) {
	cfg, observer, name := lifecycleConfig(t)
	for _, tc := range []struct {
		name   string
		change func(*config.APIConfig)
	}{
		{"invalid encryption key", func(c *config.APIConfig) { c.SecretsKey = "bad" }},
		{"missing CA", func(c *config.APIConfig) { c.CAFile = "/nonexistent/promptgate-ca.pem" }},
		{"invalid Redis URL", func(c *config.APIConfig) { c.RedisURL = "not-a-redis-url" }},
		{"invalid model lab URL", func(c *config.APIConfig) { c.ProxyBaseURL = "ftp://invalid.test" }},
		{"invalid JWKS URL", func(c *config.APIConfig) { c.KeycloakJWKSURL = ":invalid" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broken := cfg
			tc.change(&broken)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a, err := New(ctx, broken)
			if err == nil {
				_ = a.Close()
				t.Fatal("invalid configuration accepted")
			}
			assertRuntimeConnectionsClosed(t, observer, name)
		})
	}
}

func TestProxyInitializationFailureReleasesConnections(t *testing.T) {
	api, observer, name := lifecycleConfig(t)
	cfg, err := config.LoadProxy(testconfig.Path(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.DatabaseURL, cfg.RedisURL = api.DatabaseURL, api.RedisURL
	cfg.IPValidationEnabled = false
	for _, tc := range []struct {
		name   string
		change func(*config.ProxyConfig)
	}{
		{"encryption key", func(c *config.ProxyConfig) { c.SecretsKey = "invalid" }},
		{"Redis URL", func(c *config.ProxyConfig) { c.RedisURL = "invalid" }},
		{"model lab", func(c *config.ProxyConfig) { c.ProxyBaseURL = "ftp://invalid.test" }},
		{"CA file", func(c *config.ProxyConfig) { c.CAFile = "/nonexistent/promptgate-ca.pem" }},
		{"IP validation URL", func(c *config.ProxyConfig) { c.IPValidationEnabled = true; c.IPValidationURLTemplate = ":invalid" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broken := cfg
			tc.change(&broken)
			runtime, err := NewProxy(context.Background(), broken, nil, cdrslog.Make())
			if err == nil {
				_ = runtime.Close(context.Background())
				t.Fatal("invalid runtime configuration accepted")
			}
			assertRuntimeConnectionsClosed(t, observer, name)
		})
	}
}

func TestAPIRuntimeDiscoversOIDCAndPersistsAuthorizationState(t *testing.T) {
	cfg, observer, name := lifecycleConfig(t)
	var issuer *httptest.Server
	issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/jwks" {
			fmt.Fprint(w, `{"keys":[]}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "jwks_uri": issuer.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	defer issuer.Close()
	cfg.KeycloakIssuerURL = issuer.URL
	cfg.KeycloakJWKSURL = issuer.URL + "/jwks"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	if a.ModelLab == nil || a.Sessions == nil || a.OIDC == nil || a.Validator == nil || a.QuotaRedis == nil {
		t.Fatal("incomplete runtime wiring")
	}
	redirect, err := a.OIDC.AuthorizationURL("/lab", "")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(redirect)
	state := parsed.Query().Get("state")
	request, ok := a.OIDC.ConsumeAuthorizationRequest(state)
	if !ok || request.RedirectPath != "/lab" || parsed.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("authorization state or PKCE lost")
	}
	if _, ok := a.OIDC.ConsumeAuthorizationRequest(state); ok {
		t.Fatal("authorization state replayed")
	}
	cancel()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	assertRuntimeConnectionsClosed(t, observer, name)
}

func TestProxyRuntimeRoutesAuthenticatedRequestsAndReloads(t *testing.T) {
	api, observer, name := lifecycleConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := New(ctx, api)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer upstream-secret" {
			t.Error("upstream credential not applied")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/chat/completions" {
			fmt.Fprint(w, `{"id":"chatcmpl-feature","object":"chat.completion","created":1,"model":"model-a","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
			return
		}
		fmt.Fprint(w, `{"object":"list","data":[{"id":"model-a","object":"model"}]}`)
	}))
	defer upstream.Close()
	createdProvider, err := a.Providers.CreateProvider(ctx, provider.CreateProviderInput{Name: "feature", Type: provider.ProviderTypeOpenAI, BaseURL: upstream.URL + "/v1", APIKey: "upstream-secret", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	user, err := a.Users.CreateUser(ctx, "operator")
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Users.UpdateUser(ctx, user.ID, users.UpdateUserInput{Role: auth.RoleAdmin, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	group, err := a.Groups.CreateGroup(ctx, groups.CreateGroupInput{Name: "feature", DisplayName: "Feature", ProviderIDs: []string{createdProvider.ID.String()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Groups.AddMember(ctx, group.ID.String(), user.ID); err != nil {
		t.Fatal(err)
	}
	profile, err := a.Users.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	token, err := a.Tokens.CreateToken(ctx, profile, "test-key", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Subscriptions.CreatePlan(ctx, subscriptions.PlanInput{Name: "Default", IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadProxy(testconfig.Path(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.DatabaseURL = api.DatabaseURL
	cfg.RedisURL = api.RedisURL
	cfg.IPValidationEnabled = false
	cfg.ProxyBasePath = "/remote"
	runtime, err := NewProxy(ctx, cfg, nil, cdrslog.Make())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(context.Background())
	runtime.Start(ctx)
	for _, tc := range []struct {
		path, token string
		status      int
	}{
		{"/live", "", 200}, {"/health", "", 200}, {"/remote/feature/v1/models", "", 401}, {"/remote/feature/v1/models", "invalid", 401}, {"/remote/feature/v1/models", token.Token, 200}, {"/feature/v1/models", token.Token, 404},
	} {
		req := httptest.NewRequest("GET", tc.path, nil)
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		rec := httptest.NewRecorder()
		runtime.Handler.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s: %d want %d: %s", tc.path, rec.Code, tc.status, rec.Body.String())
		}
		if tc.status == 200 && strings.Contains(tc.path, "models") && !strings.Contains(rec.Body.String(), "model-a") {
			t.Fatal("upstream models missing")
		}
	}
	if err := runtime.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	worker := usage.NewWorker(a.DB, runtime.redis, usage.WorkerOptions{ConsumerName: "feature", BatchSize: 10, BlockTimeout: 100 * time.Millisecond, PendingIdleTimeout: time.Second}, nil)
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	var workerErr error
	go func() { workerErr = worker.Run(workerCtx); close(workerDone) }()
	stopAndWait := func() {
		stopWorker()
		select {
		case <-workerDone:
			if workerErr != nil {
				t.Error(workerErr)
			}
		case <-time.After(3 * time.Second):
			t.Error("usage worker did not stop")
		}
	}
	defer stopAndWait()
	req := httptest.NewRequest("POST", "/remote/feature/v1/chat/completions", strings.NewReader(`{"model":"model-a","messages":[{"role":"user","content":"Hello"}]}`))
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	runtime.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("chat request: %d %s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		totals, err := a.Proxy.DashboardTokens(ctx, user.ID, usage.UsageWindowAll, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if totals.TotalTokens == 15 {
			account, err := a.Users.GetUser(ctx, user.ID)
			if err != nil || account.TotalTokens != totals.TotalTokens || totals.InputTokens != 10 || totals.OutputTokens != 5 {
				t.Fatalf("dashboard/account mismatch: %#v %#v %v", totals, account, err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("HTTP usage did not reach durable KPIs: %#v", totals)
		}
		time.Sleep(10 * time.Millisecond)
	}
	stopAndWait()
	cancel()
	if err := runtime.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	assertRuntimeConnectionsClosed(t, observer, name)
}
