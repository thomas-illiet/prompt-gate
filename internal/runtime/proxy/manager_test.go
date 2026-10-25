package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/secrets"

	"github.com/alicebob/miniredis/v2"
	coderbridge "github.com/coder/aibridge"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeManagedBridge struct {
	served   bool
	shutdown chan struct{}
	headers  http.Header
	trailers http.Header
}

func (b *fakeManagedBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.served = true
	b.headers = r.Header.Clone()
	b.trailers = r.Trailer.Clone()
	w.WriteHeader(http.StatusNoContent)
}

func (b *fakeManagedBridge) Shutdown(context.Context) error {
	if b.shutdown != nil {
		close(b.shutdown)
	}
	return nil
}

func TestManagerDelegatesAndShutsDownManagedBridge(t *testing.T) {
	bridge := &fakeManagedBridge{}
	manager := &Manager{opts: Options{Logger: slog.Default()}}
	manager.installBridge(bridge)

	response := httptest.NewRecorder()
	manager.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusNoContent || !bridge.served {
		t.Fatalf("expected active bridge to serve request, status=%d served=%v", response.Code, bridge.served)
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown manager: %v", err)
	}
}

func TestManagerAllowsOnlyExplicitUpstreamHeaders(t *testing.T) {
	bridge := &fakeManagedBridge{}
	manager := &Manager{opts: Options{Logger: slog.Default()}}
	manager.installBridge(bridge)

	request := httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	for name, value := range map[string]string{
		"Accept":              "text/event-stream",
		"Accept-Encoding":     "identity",
		"Anthropic-Beta":      "prompt-caching-2024-07-31",
		"Anthropic-Version":   "2023-06-01",
		"Content-Type":        "application/json",
		"Idempotency-Key":     "request-123",
		"OpenAI-Beta":         "responses=v1",
		"User-Agent":          "private-client/1.0",
		"X-Client-Request-Id": "internal-request-123",
		"Authorization":       "Bearer promptgate-secret",
		"X-Api-Key":           "provider-secret",
		"Cookie":              "promptgate_session=session-secret",
		"X-Admin-API-Key":     "admin-secret",
		"Forwarded":           "for=198.51.100.10",
		"X-Forwarded-For":     "198.51.100.10",
		"X-Forwarded-Host":    "internal.example.test",
		"X-Forwarded-Proto":   "https",
		"X-Real-IP":           "198.51.100.10",
		"Traceparent":         "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01",
		"Tracestate":          "vendor=value",
		"Baggage":             "secret=value",
		"X-Session-Id":        "session-id",
		"OpenAI-Organization": "org-secret",
		"OpenAI-Project":      "project-secret",
		"X-Custom-Secret":     "custom-secret",
	} {
		request.Header.Set(name, value)
	}
	request.Header.Add("Anthropic-Beta", "second-beta")
	request.Trailer = http.Header{
		"Cookie":          {"trailer-session-secret"},
		"X-Custom-Secret": {"trailer-custom-secret"},
	}
	originalHeaders := request.Header.Clone()
	response := httptest.NewRecorder()

	manager.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", response.Code, http.StatusNoContent)
	}
	for name, want := range map[string]string{
		"Accept":            "text/event-stream",
		"Accept-Encoding":   "identity",
		"Anthropic-Version": "2023-06-01",
		"Content-Type":      "application/json",
		"OpenAI-Beta":       "responses=v1",
	} {
		if got := bridge.headers.Get(name); got != want {
			t.Errorf("allowed header %s: got %q, want %q", name, got, want)
		}
	}
	if got := bridge.headers.Values("Anthropic-Beta"); len(got) != 2 || got[0] != "prompt-caching-2024-07-31" || got[1] != "second-beta" {
		t.Fatalf("Anthropic-Beta values were not preserved: %#v", got)
	}
	if got, want := len(bridge.headers), len(upstreamHeaderAllowlist); got != want {
		t.Fatalf("upstream headers: got %#v (%d), want only %d allowlisted names", bridge.headers, got, want)
	}
	if len(bridge.trailers) != 0 {
		t.Fatalf("upstream trailers were not removed: %#v", bridge.trailers)
	}
	if got := bridge.headers.Get("Idempotency-Key"); got != "" {
		t.Fatalf("client idempotency key crossed the shared provider credential boundary: %q", got)
	}
	if request.Header.Get("Cookie") != originalHeaders.Get("Cookie") || request.Header.Get("X-Custom-Secret") != originalHeaders.Get("X-Custom-Secret") {
		t.Fatal("manager mutated the caller's request headers")
	}
}

func TestManagerShutsDownPreviousBridgeAfterSwap(t *testing.T) {
	previous := &fakeManagedBridge{shutdown: make(chan struct{})}
	manager := &Manager{opts: Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	manager.installBridge(previous)
	manager.installBridge(&fakeManagedBridge{})

	select {
	case <-previous.shutdown:
	case <-time.After(time.Second):
		t.Fatal("previous bridge was not shut down after swap")
	}
}

// TestRebuildConstructsBridgeFromProvidersOnly verifies the retired MCP runtime is not part of bridge construction.
func TestRebuildConstructsBridgeFromProvidersOnly(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	providerService := provider.NewService(db, cipher)
	if err := providerService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate providers: %v", err)
	}
	if _, err := providerService.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "openai",
		Type:    provider.ProviderTypeOpenAI,
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	manager := &Manager{opts: Options{Providers: providerService, Logger: slog.Default()}}
	manager.buildBridge = func(_ context.Context, providers []coderbridge.Provider) (managedBridge, error) {
		if len(providers) != 1 {
			t.Fatalf("providers: got %d, want 1", len(providers))
		}
		return &fakeManagedBridge{}, nil
	}
	if err := manager.Rebuild(ctx); err != nil {
		t.Fatalf("rebuild bridge: %v", err)
	}
}

func TestBuildProvidersDisableRawAPIDumps(t *testing.T) {
	t.Setenv("BRIDGE_DUMP_DIR", t.TempDir())

	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	providerService := provider.NewService(db, cipher)
	if err := providerService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate providers: %v", err)
	}
	for _, input := range []provider.CreateProviderInput{
		{Name: "openai-no-dump", Type: provider.ProviderTypeOpenAI, BaseURL: "https://api.openai.com/v1", APIKey: "openai-key", Enabled: true},
		{Name: "anthropic-no-dump", Type: provider.ProviderTypeAnthropic, BaseURL: "https://api.anthropic.com", APIKey: "anthropic-key", Enabled: true},
	} {
		if _, err := providerService.CreateProvider(ctx, input); err != nil {
			t.Fatalf("create provider %q: %v", input.Name, err)
		}
	}

	manager := &Manager{opts: Options{Providers: providerService, Logger: slog.Default()}}
	manager.buildBridge = func(_ context.Context, providers []coderbridge.Provider) (managedBridge, error) {
		if len(providers) != 2 {
			t.Fatalf("providers: got %d, want 2", len(providers))
		}
		for _, built := range providers {
			if dumpDir := built.APIDumpDir(); dumpDir != "" {
				t.Fatalf("provider %q retained raw API dump directory %q", built.Name(), dumpDir)
			}
		}
		return &fakeManagedBridge{}, nil
	}
	if err := manager.Rebuild(ctx); err != nil {
		t.Fatalf("rebuild bridge: %v", err)
	}
	if _, present := os.LookupEnv("BRIDGE_DUMP_DIR"); present {
		t.Fatal("legacy AI Bridge dump environment variable was not cleared")
	}
}

func TestReloadDebouncerCoalescesSchedules(t *testing.T) {
	debouncer := newReloadDebouncer(time.Millisecond)
	defer debouncer.Stop()
	if debouncer.Schedule() {
		t.Fatal("first schedule must not be reported as a reschedule")
	}
	if !debouncer.Schedule() {
		t.Fatal("second schedule must reset the active timer")
	}
	select {
	case <-debouncer.C():
		debouncer.Clear()
	case <-time.After(time.Second):
		t.Fatal("debounced reload did not fire")
	}
}

// TestWatchReloadsAccessGroupsFromRedisEvent verifies watch reloads access groups from Redis event.
func TestWatchDoesNotTrustCachedGroupsWithoutAuthoritativeService(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	store, err := redisstore.NewRequired(ctx, "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	defer store.Close()

	snapshot := groups.Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes: map[string]provider.ProviderType{
			"openai": provider.ProviderTypeOpenAI,
		},
		Users: map[string]groups.UserAccess{
			"user-id": {
				Rules: []groups.AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`^any-`},
				}},
			},
		},
		UserGroups: map[string][]string{"user-id": {"engineering"}},
	}
	if err := store.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainGroups), snapshot, time.Minute); err != nil {
		t.Fatalf("cache group snapshot: %v", err)
	}

	accessStore := groups.NewSnapshotStore(nil)
	manager := &Manager{opts: Options{
		AccessSnapshot: accessStore,
		Redis:          store,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}}

	version := int64(1)
	if err := manager.reconcileGroups(ctx, &version, false); err == nil {
		t.Fatal("expected authoritative group service error")
	}
	if accessStore.Allows("user-id", "openai", "any-model") {
		t.Fatal("cached group grants were trusted without an authoritative service")
	}
}

// TestRefreshAccessGroupsFallsBackWhenRedisSnapshotIsLegacy verifies refresh access groups falls back when Redis snapshot is legacy.
func TestRefreshAccessGroupsFallsBackWhenRedisSnapshotIsLegacy(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	store, err := redisstore.NewRequired(ctx, "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	defer store.Close()

	legacySnapshot := groups.Snapshot{
		KnownProviders: []string{"openai"},
		Users: map[string]groups.UserAccess{
			"user-id": {
				Rules: []groups.AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`^legacy-`},
				}},
			},
		},
	}
	if err := store.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainGroups), legacySnapshot, time.Minute); err != nil {
		t.Fatalf("cache legacy group snapshot: %v", err)
	}

	groupService, userID := newManagerGroupService(t)
	accessStore := groups.NewSnapshotStore(groupService)
	manager := &Manager{opts: Options{
		AccessSnapshot: accessStore,
		Redis:          store,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}}

	if err := manager.RefreshAccessGroups(ctx); err != nil {
		t.Fatalf("refresh access groups: %v", err)
	}
	if !accessStore.Allows(userID, "openai", "gpt-5-mini") {
		t.Fatal("expected SQL snapshot fallback to allow configured group access")
	}
	if accessStore.Allows("user-id", "openai", "legacy-model") {
		t.Fatal("legacy Redis snapshot should not be installed")
	}
}

// newManagerGroupService creates manager group service.
func newManagerGroupService(t *testing.T) (*groups.Service, string) {
	t.Helper()
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&users.User{}, &provider.Provider{}); err != nil {
		t.Fatalf("migrate dependencies: %v", err)
	}
	groupService := groups.NewService(db)
	if err := groupService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate groups: %v", err)
	}

	userID := uuid.NewString()
	if err := db.Create(&users.User{
		ID:                userID,
		ExternalSub:       testPointer("oidc-sub"),
		Email:             "user@example.com",
		PreferredUsername: "user",
		Name:              "User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       testPointer(time.Now().UTC()),
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	providerRecord := provider.Provider{
		ID:          uuid.New(),
		Name:        "openai",
		DisplayName: "OpenAI",
		Type:        provider.ProviderTypeOpenAI,
		BaseURL:     "https://api.openai.com/v1",
		Enabled:     true,
	}
	if err := db.Create(&providerRecord).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	group, err := groupService.CreateGroup(ctx, groups.CreateGroupInput{
		Name:          "engineering",
		DisplayName:   "Engineering",
		ProviderIDs:   []string{providerRecord.ID.String()},
		ModelPatterns: []string{`^gpt-5`},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := groupService.ReplaceUserGroups(ctx, userID, []string{group.ID.String()}); err != nil {
		t.Fatalf("replace user groups: %v", err)
	}
	return groupService, userID
}
