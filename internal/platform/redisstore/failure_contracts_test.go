package redisstore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestCacheCorruptionCannotAuthenticateOrReplayState(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	store, err := NewRequired(ctx, "redis://"+server.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var value map[string]string
	for _, version := range []string{"-1", "not-a-number"} {
		server.Set(VersionKey("auth"), version)
		if _, found, err := store.GetAuthCacheJSON(ctx, "token", &value); err == nil || found {
			t.Fatalf("corrupt version %q accepted", version)
		}
	}
	server.Set(VersionKey("auth"), "2")
	server.Set(AuthCacheKey(2, "token"), "broken JSON")
	if _, found, err := store.GetAuthCacheJSON(ctx, "token", &value); err == nil || found {
		t.Fatal("corrupt principal accepted")
	}
	server.Set(AuthRequestKey("state"), "broken JSON")
	if _, err := store.GetDelJSON(ctx, AuthRequestKey("state"), &value); err == nil {
		t.Fatal("corrupt state accepted")
	}
	if found, err := store.GetDelJSON(ctx, AuthRequestKey("state"), &value); err != nil || found {
		t.Fatal("consumed state replayed")
	}
	if err := store.SetJSON(ctx, "bad", make(chan int), 0); err == nil {
		t.Fatal("unserializable cache value accepted")
	}
	if server.Exists("bad") {
		t.Fatal("failed serialization left a value")
	}
	if err := store.SetJSON(ctx, "expires", map[string]string{"value": "safe"}, 0); err != nil {
		t.Fatal(err)
	}
	server.FastForward(time.Minute)
	if found, err := store.GetJSON(ctx, "expires", &value); err != nil || found {
		t.Fatal("expired cache value returned")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetJSON(ctx, "key", &value); err == nil {
		t.Fatal("storage error hidden as cache miss")
	}
	if _, err := store.GetDelJSON(ctx, "key", &value); err == nil {
		t.Fatal("storage error hidden as missing state")
	}
	if _, _, err := store.GetAuthCacheJSON(ctx, "key", &value); err == nil {
		t.Fatal("storage error hidden during authentication")
	}
	if _, err := store.Version(ctx, "auth"); err == nil {
		t.Fatal("storage error hidden during invalidation")
	}
	store.Notify(ctx, "auth") // Failure must be logged without panicking.
	if _, open := <-store.Subscribe(ctx); open {
		t.Fatal("subscription remained open after connection failure")
	}
}

func TestMalformedEventDoesNotPreventLaterInvalidation(t *testing.T) {
	server := miniredis.RunT(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store, err := NewRequired(ctx, "redis://"+server.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	events := store.Subscribe(ctx)
	server.Set(SnapshotKey("subscriptions"), `{"stale":true}`)
	if err := store.Client().Publish(ctx, EventsChannel, "not JSON").Err(); err != nil {
		t.Fatal(err)
	}
	store.Notify(ctx, "subscriptions")
	select {
	case event := <-events:
		if event.Domain != "subscriptions" || event.Version != 1 {
			t.Fatalf("wrong invalidation: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("malformed event stalled subscriber")
	}
	if server.Exists(SnapshotKey("subscriptions")) {
		t.Fatal("stale subscription snapshot survived invalidation")
	}
	if err := store.Del(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("event after cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not terminate")
	}
}

func TestOptionalDisabledCacheContract(t *testing.T) {
	var store *Store
	ctx := context.Background()
	var value any
	if store.Enabled() || store.Client() != nil || store.TTL() != 5*time.Minute {
		t.Fatal("invalid disabled cache defaults")
	}
	if found, err := store.GetJSON(ctx, "key", &value); found || err != nil {
		t.Fatal("disabled cache read")
	}
	if _, found, err := store.GetAuthCacheJSON(ctx, "key", &value); found || err != nil {
		t.Fatal("disabled auth cache read")
	}
	if found, err := store.GetDelJSON(ctx, "key", &value); found || err != nil {
		t.Fatal("disabled state read")
	}
	if err := store.SetJSON(ctx, "key", value, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Del(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if version, err := store.Version(ctx, "auth"); version != 0 || err != nil {
		t.Fatal("disabled version")
	}
	store.Notify(ctx, "auth")
	if _, ok := <-store.Subscribe(ctx); ok {
		t.Fatal("disabled subscription blocked")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
