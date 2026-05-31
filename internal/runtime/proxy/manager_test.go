package runtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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
