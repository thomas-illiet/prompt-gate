package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
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

type watcherFixture struct {
	db            *gorm.DB
	store         *redisstore.Store
	manager       *Manager
	provider      provider.ProviderResponse
	group         groups.GroupResponse
	groupService  *groups.Service
	userID        string
	bridgeBuilds  chan string
	reconcileWait time.Duration
}

func newWatcherFixture(t *testing.T) watcherFixture {
	t.Helper()
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&users.User{}, &provider.Provider{}); err != nil {
		t.Fatalf("migrate watcher dependencies: %v", err)
	}
	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatalf("create watcher cipher: %v", err)
	}
	providerService := provider.NewService(db, cipher)
	if err := providerService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate watcher providers: %v", err)
	}
	createdProvider, err := providerService.CreateProvider(ctx, provider.CreateProviderInput{
		Name: "watcher-openai", Type: provider.ProviderTypeOpenAI,
		BaseURL: "https://old.example.test/v1", APIKey: "test-key", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create watcher provider: %v", err)
	}

	userID := uuid.NewString()
	externalSub := "watcher-user-sub"
	if err := db.Create(&users.User{
		ID: userID, ExternalSub: &externalSub, PreferredUsername: "watcher-user",
		Type: auth.UserTypeUser, Role: auth.RoleUser, IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create watcher user: %v", err)
	}
	groupService := groups.NewService(db)
	if err := groupService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate watcher groups: %v", err)
	}
	createdGroup, err := groupService.CreateGroup(ctx, groups.CreateGroupInput{
		Name: "watcher-group", DisplayName: "Watcher group",
		ProviderIDs: []string{createdProvider.ID.String()}, ModelPatterns: []string{`^gpt-`},
	})
	if err != nil {
		t.Fatalf("create watcher group: %v", err)
	}
	if _, err := groupService.ReplaceUserGroups(ctx, userID, []string{createdGroup.ID.String()}); err != nil {
		t.Fatalf("assign watcher group: %v", err)
	}

	redisServer := miniredis.RunT(t)
	store, err := redisstore.NewRequired(ctx, "redis://"+redisServer.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("create watcher redis store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	bridgeBuilds := make(chan string, 32)
	manager := &Manager{opts: Options{
		Providers: providerService, AccessSnapshot: groups.NewSnapshotStore(groupService), Redis: store,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ReloadDebounce: time.Millisecond,
		ConfigReconcileInterval: 10 * time.Millisecond, ConfigSubscribeRetry: 5 * time.Millisecond,
	}}
	manager.initializeVersions()
	manager.buildBridge = func(_ context.Context, providers []coderbridge.Provider) (managedBridge, error) {
		baseURL := ""
		if len(providers) > 0 {
			baseURL = providers[0].BaseURL()
		}
		bridgeBuilds <- baseURL
		return &fakeManagedBridge{}, nil
	}
	if err := manager.RefreshAccessGroups(ctx); err != nil {
		t.Fatalf("load initial group snapshot: %v", err)
	}
	if err := manager.Rebuild(ctx); err != nil {
		t.Fatalf("build initial proxy bridge: %v", err)
	}
	waitForWatcherBuild(t, bridgeBuilds, "https://old.example.test/v1")

	return watcherFixture{
		db: db, store: store, manager: manager, provider: createdProvider, group: createdGroup,
		groupService: groupService, userID: userID, bridgeBuilds: bridgeBuilds,
		reconcileWait: 2 * time.Second,
	}
}

func TestWatchReconcilesMissedVersionsAndDisablesLastProvider(t *testing.T) {
	fixture := newWatcherFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go fixture.manager.Watch(ctx)

	// The first subscription always performs an authoritative catch-up after it
	// is established, closing the startup race even when the version is still 0.
	waitForWatcherCondition(t, fixture.reconcileWait, func() bool {
		return fixture.manager.providerVersion.Load() == 0 && fixture.manager.groupVersion.Load() == 0
	}, "initial watcher reconciliation did not complete")

	const updatedBaseURL = "https://new.example.test/v1"
	if err := fixture.db.Model(&provider.Provider{}).
		Where("id = ?", fixture.provider.ID).
		Update("base_url", updatedBaseURL).Error; err != nil {
		t.Fatalf("update provider without publication: %v", err)
	}
	patterns := []string{`^claude-`}
	if _, err := fixture.groupService.UpdateGroup(context.Background(), fixture.group.ID.String(), groups.UpdateGroupInput{
		ModelPatterns: &patterns,
	}); err != nil {
		t.Fatalf("update group without publication: %v", err)
	}
	// Neither a version increment nor a Pub/Sub message is sent. The periodic
	// authoritative reconciliation must still catch up after a total notify loss.
	waitForWatcherBuild(t, fixture.bridgeBuilds, updatedBaseURL)
	waitForWatcherCondition(t, fixture.reconcileWait, func() bool {
		return fixture.manager.opts.AccessSnapshot.Allows(fixture.userID, fixture.provider.Name, "claude-sonnet") &&
			!fixture.manager.opts.AccessSnapshot.Allows(fixture.userID, fixture.provider.Name, "gpt-test")
	}, "group snapshot did not reconcile after a missed publication")

	if err := fixture.db.Model(&provider.Provider{}).
		Where("id = ?", fixture.provider.ID).
		Update("enabled", false).Error; err != nil {
		t.Fatalf("disable last provider without publication: %v", err)
	}
	waitForWatcherCondition(t, fixture.reconcileWait, func() bool {
		entry, _ := fixture.manager.current.Load().(*bridgeEntry)
		return entry != nil && entry.bridge == nil
	}, "last-provider disable kept the old bridge active")
}

func TestNewManagerIgnoresStaleRedisSnapshotsAtStartup(t *testing.T) {
	fixture := newWatcherFixture(t)
	ctx := context.Background()

	staleProviders, err := fixture.manager.opts.Providers.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("load provider records for stale snapshot: %v", err)
	}
	staleProviders[0].BaseURL = "https://stale-cache.example.test/v1"
	if err := fixture.store.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainProviders), staleProviders, time.Minute); err != nil {
		t.Fatalf("cache stale provider snapshot: %v", err)
	}
	staleGroups := groups.Snapshot{
		KnownProviders: []string{fixture.provider.Name},
		ProviderTypes:  map[string]provider.ProviderType{fixture.provider.Name: provider.ProviderTypeOpenAI},
		Users: map[string]groups.UserAccess{
			fixture.userID: {
				Rules: []groups.AccessRule{{
					Providers: []string{fixture.provider.Name}, ModelPatterns: []string{`^stale-`},
				}},
			},
		},
		UserGroups: map[string][]string{fixture.userID: {"stale-group"}},
	}
	if err := fixture.store.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainGroups), staleGroups, time.Minute); err != nil {
		t.Fatalf("cache stale group snapshot: %v", err)
	}

	accessSnapshot := groups.NewSnapshotStore(fixture.groupService)
	manager, err := NewManager(ctx, Options{
		Providers: fixture.manager.opts.Providers, AccessSnapshot: accessSnapshot, Redis: fixture.store,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("create manager from authoritative startup state: %v", err)
	}
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })

	if len(manager.appliedProviders) != 1 || manager.appliedProviders[0].BaseURL != fixture.provider.BaseURL {
		t.Fatalf("startup installed stale provider snapshot: %#v", manager.appliedProviders)
	}
	if !accessSnapshot.Allows(fixture.userID, fixture.provider.Name, "gpt-test") {
		t.Fatal("startup did not load the authoritative group snapshot")
	}
	if accessSnapshot.Allows(fixture.userID, fixture.provider.Name, "stale-model") {
		t.Fatal("startup installed the stale Redis group snapshot")
	}
}

func TestWatchResubscribesAfterEventChannelCloses(t *testing.T) {
	fixture := newWatcherFixture(t)
	var subscriptions atomic.Int32
	liveEvents := make(chan redisstore.Event)
	fixture.manager.subscribeEvents = func(context.Context) <-chan redisstore.Event {
		if subscriptions.Add(1) == 1 {
			closed := make(chan redisstore.Event)
			close(closed)
			return closed
		}
		return liveEvents
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go fixture.manager.Watch(ctx)
	waitForWatcherCondition(t, fixture.reconcileWait, func() bool {
		return subscriptions.Load() >= 2
	}, "watcher did not resubscribe after the event channel closed")

	const updatedBaseURL = "https://after-reconnect.example.test/v1"
	if err := fixture.db.Model(&provider.Provider{}).
		Where("id = ?", fixture.provider.ID).
		Update("base_url", updatedBaseURL).Error; err != nil {
		t.Fatalf("update provider after resubscribe: %v", err)
	}
	if err := fixture.store.Client().Incr(context.Background(), redisstore.VersionKey(configevents.DomainProviders)).Err(); err != nil {
		t.Fatalf("increment provider version after resubscribe: %v", err)
	}
	waitForWatcherBuild(t, fixture.bridgeBuilds, updatedBaseURL)
}

func TestWatchReconcilesWhileSubscriptionsKeepClosing(t *testing.T) {
	fixture := newWatcherFixture(t)
	var subscriptions atomic.Int32
	fixture.manager.subscribeEvents = func(context.Context) <-chan redisstore.Event {
		subscriptions.Add(1)
		closed := make(chan redisstore.Event)
		close(closed)
		return closed
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go fixture.manager.Watch(ctx)
	waitForWatcherCondition(t, fixture.reconcileWait, func() bool {
		return subscriptions.Load() >= 2
	}, "watcher did not retry continuously closed subscriptions")

	const updatedBaseURL = "https://closed-subscription.example.test/v1"
	if err := fixture.db.Model(&provider.Provider{}).
		Where("id = ?", fixture.provider.ID).
		Update("base_url", updatedBaseURL).Error; err != nil {
		t.Fatalf("update provider while subscriptions are closed: %v", err)
	}
	// There is intentionally no version increment or publication. Wall-clock
	// reconciliation must continue even if every event channel closes immediately.
	waitForWatcherBuild(t, fixture.bridgeBuilds, updatedBaseURL)
}

func TestAuthoritativeProviderReconcileFailsClosedAfterBuildError(t *testing.T) {
	fixture := newWatcherFixture(t)
	originalBuilder := fixture.manager.buildBridge
	if err := fixture.db.Model(&provider.Provider{}).
		Where("id = ?", fixture.provider.ID).
		Update("base_url", "https://invalid-change.example.test/v1").Error; err != nil {
		t.Fatalf("change authoritative provider config: %v", err)
	}
	errBuild := errors.New("provider build failed")
	fixture.manager.buildBridge = func(context.Context, []coderbridge.Provider) (managedBridge, error) {
		return nil, errBuild
	}

	err := fixture.manager.reconcileProviders(context.Background(), nil, true)
	if !errors.Is(err, errBuild) {
		t.Fatalf("expected provider build error, got %v", err)
	}
	entry, _ := fixture.manager.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge != nil {
		t.Fatal("authoritative build failure retained the old provider bridge")
	}

	if err := fixture.db.Model(&provider.Provider{}).
		Where("id = ?", fixture.provider.ID).
		Update("base_url", fixture.provider.BaseURL).Error; err != nil {
		t.Fatalf("restore last valid provider config: %v", err)
	}
	fixture.manager.buildBridge = originalBuilder
	if err := fixture.manager.reconcileProviders(context.Background(), nil, true); err != nil {
		t.Fatalf("recover after provider config revert: %v", err)
	}
	waitForWatcherBuild(t, fixture.bridgeBuilds, fixture.provider.BaseURL)
	entry, _ = fixture.manager.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge == nil {
		t.Fatal("reverting to the last valid provider config left the proxy unavailable")
	}
}

func TestAuthoritativeProviderReconcileFailsClosedWhenAppliedVersionIsUnknown(t *testing.T) {
	fixture := newWatcherFixture(t)
	sqlDB, err := fixture.db.DB()
	if err != nil {
		t.Fatalf("open watcher SQL handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close watcher database: %v", err)
	}

	if err := fixture.manager.reconcileProviders(context.Background(), nil, true); err == nil {
		t.Fatal("expected authoritative provider read failure")
	}
	entry, _ := fixture.manager.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge != nil {
		t.Fatal("database read failure retained a bridge not tied to a durable version")
	}
}

func TestKnownProviderVersionChangeFailsClosedOnDatabaseErrorAndRecovers(t *testing.T) {
	fixture := newWatcherFixture(t)
	fixture.manager.providerVersion.Store(0)
	if err := fixture.store.Client().Incr(context.Background(), redisstore.VersionKey(configevents.DomainProviders)).Err(); err != nil {
		t.Fatalf("advance provider version: %v", err)
	}

	errKnownChange := errors.New("provider database unavailable after known change")
	var failReads atomic.Bool
	failReads.Store(true)
	callbackName := "test:known-provider-change"
	if err := fixture.db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if failReads.Load() {
			tx.AddError(errKnownChange)
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() { _ = fixture.db.Callback().Query().Remove(callbackName) })

	if err := fixture.manager.reconcileProviders(context.Background(), nil, false); !errors.Is(err, errKnownChange) {
		t.Fatalf("provider reconcile error = %v, want %v", err, errKnownChange)
	}
	entry, _ := fixture.manager.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge != nil {
		t.Fatal("known provider version change kept the stale bridge after database failure")
	}

	failReads.Store(false)
	if err := fixture.manager.reconcileProviders(context.Background(), nil, false); err != nil {
		t.Fatalf("recover provider reconcile: %v", err)
	}
	entry, _ = fixture.manager.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge == nil {
		t.Fatal("provider bridge did not recover after the database returned")
	}
}

func TestKnownGroupVersionChangeFailsClosedOnDatabaseErrorAndRecovers(t *testing.T) {
	fixture := newWatcherFixture(t)
	fixture.manager.groupVersion.Store(0)
	if err := fixture.store.Client().Incr(context.Background(), redisstore.VersionKey(configevents.DomainGroups)).Err(); err != nil {
		t.Fatalf("advance group version: %v", err)
	}

	errKnownChange := errors.New("group database unavailable after known change")
	var failReads atomic.Bool
	failReads.Store(true)
	callbackName := "test:known-group-change"
	if err := fixture.db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if failReads.Load() {
			tx.AddError(errKnownChange)
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() { _ = fixture.db.Callback().Query().Remove(callbackName) })

	if err := fixture.manager.reconcileGroups(context.Background(), nil, false); !errors.Is(err, errKnownChange) {
		t.Fatalf("group reconcile error = %v, want %v", err, errKnownChange)
	}
	if fixture.manager.opts.AccessSnapshot.Allows(fixture.userID, fixture.provider.Name, "gpt-test") {
		t.Fatal("known group version change kept stale grants after database failure")
	}

	failReads.Store(false)
	if err := fixture.manager.reconcileGroups(context.Background(), nil, false); err != nil {
		t.Fatalf("recover group reconcile: %v", err)
	}
	if !fixture.manager.opts.AccessSnapshot.Allows(fixture.userID, fixture.provider.Name, "gpt-test") {
		t.Fatal("group grants did not recover after the database returned")
	}
}

func TestGroupReconcileFailsClosedWhenAppliedVersionIsUnknown(t *testing.T) {
	fixture := newWatcherFixture(t)
	if fixture.manager.groupVersion.Load() != -1 {
		t.Fatalf("test requires unknown applied version, got %d", fixture.manager.groupVersion.Load())
	}
	errUnknownGeneration := errors.New("group database unavailable with unknown generation")
	callbackName := "test:unknown-group-generation"
	if err := fixture.db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		tx.AddError(errUnknownGeneration)
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() { _ = fixture.db.Callback().Query().Remove(callbackName) })

	if err := fixture.manager.reconcileGroups(context.Background(), nil, false); !errors.Is(err, errUnknownGeneration) {
		t.Fatalf("group reconcile error = %v, want %v", err, errUnknownGeneration)
	}
	if fixture.manager.opts.AccessSnapshot.Allows(fixture.userID, fixture.provider.Name, "gpt-test") {
		t.Fatal("unknown group generation kept stale grants after database failure")
	}
}

func TestKnownConfigChangeDatabaseReadIsDeadlineBound(t *testing.T) {
	fixture := newWatcherFixture(t)
	fixture.manager.providerVersion.Store(0)
	fixture.manager.opts.ConfigLoadTimeout = 25 * time.Millisecond
	if err := fixture.store.Client().Incr(context.Background(), redisstore.VersionKey(configevents.DomainProviders)).Err(); err != nil {
		t.Fatalf("advance provider version: %v", err)
	}

	callbackName := "test:block-provider-query"
	if err := fixture.db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		<-tx.Statement.Context.Done()
		tx.AddError(tx.Statement.Context.Err())
	}); err != nil {
		t.Fatalf("register blocking query callback: %v", err)
	}

	started := time.Now()
	err := fixture.manager.reconcileProviders(context.Background(), nil, false)
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("database reconciliation ignored its deadline: %s", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reconcile error = %v, want deadline exceeded", err)
	}
	entry, _ := fixture.manager.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge != nil {
		t.Fatal("known provider change retained a stale bridge after database timeout")
	}
	if err := fixture.db.Callback().Query().Remove(callbackName); err != nil {
		t.Fatalf("remove blocking callback: %v", err)
	}
	if err := fixture.manager.reconcileProviders(context.Background(), nil, false); err != nil {
		t.Fatalf("reconcile after timeout: %v", err)
	}
}

func TestAuthoritativeProviderReconcileFailsClosedAfterDecryptError(t *testing.T) {
	fixture := newWatcherFixture(t)
	if err := fixture.db.Model(&provider.Provider{}).
		Where("id = ?", fixture.provider.ID).
		Update("api_key_ciphertext", "invalid-ciphertext").Error; err != nil {
		t.Fatalf("corrupt authoritative provider credential: %v", err)
	}

	err := fixture.manager.reconcileProviders(context.Background(), nil, true)
	if err == nil || !strings.Contains(err.Error(), "decrypt provider") {
		t.Fatalf("expected provider decrypt error, got %v", err)
	}
	entry, _ := fixture.manager.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge != nil {
		t.Fatal("authoritative decrypt failure retained the old provider bridge")
	}
}

func waitForWatcherBuild(t *testing.T, builds <-chan string, want string) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case got := <-builds:
			if got == want {
				return
			}
		case <-deadline.C:
			t.Fatalf("timed out waiting for bridge build %q", want)
		}
	}
}

func waitForWatcherCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal(message)
}
