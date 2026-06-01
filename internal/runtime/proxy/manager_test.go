package runtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func TestWatchReloadsAccessGroupsFromRedisEvent(t *testing.T) {
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

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go manager.Watch(watchCtx)

	redisClient := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	defer redisClient.Close()
	event := redisstore.Event{
		Domain:    configevents.DomainGroups,
		Version:   1,
		CreatedAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := redisClient.Publish(ctx, redisstore.EventsChannel, payload).Err(); err != nil {
			t.Fatalf("publish event: %v", err)
		}
		if accessStore.Allows("user-id", "openai", "any-model") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for group access snapshot reload")
}

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
		ExternalSub:       "oidc-sub",
		Email:             "user@example.com",
		PreferredUsername: "user",
		Name:              "User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
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
