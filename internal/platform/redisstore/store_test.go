package redisstore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// TestStoreNotifyVersionAndSubscribe verifies Redis version bumps and event delivery.
func TestStoreNotifyVersionAndSubscribe(t *testing.T) {
	srv := miniredis.RunT(t)
	store, err := NewRequired(context.Background(), "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	defer store.Close()

	events := store.Subscribe(context.Background())
	store.Notify(context.Background(), "firewall")

	select {
	case event := <-events:
		if event.Domain != "firewall" || event.Version != 1 {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for redis event")
	}

	version, err := store.Version(context.Background(), "firewall")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected version 1, got %d", version)
	}
}

// TestStoreJSON verifies JSON cache values can be stored and loaded.
func TestStoreJSON(t *testing.T) {
	srv := miniredis.RunT(t)
	store, err := NewRequired(context.Background(), "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	defer store.Close()

	value := map[string]string{"hello": "world"}
	if err := store.SetJSON(context.Background(), "key", value, time.Minute); err != nil {
		t.Fatalf("set json: %v", err)
	}
	var got map[string]string
	ok, err := store.GetJSON(context.Background(), "key", &got)
	if err != nil {
		t.Fatalf("get json: %v", err)
	}
	if !ok || got["hello"] != "world" {
		t.Fatalf("unexpected value: ok=%v got=%#v", ok, got)
	}
}

// TestStoreGetDelJSON verifies JSON values can be atomically loaded and removed.
func TestStoreGetDelJSON(t *testing.T) {
	srv := miniredis.RunT(t)
	store, err := NewRequired(context.Background(), "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	defer store.Close()

	value := map[string]string{"hello": "world"}
	if err := store.SetJSON(context.Background(), "key", value, time.Minute); err != nil {
		t.Fatalf("set json: %v", err)
	}
	var got map[string]string
	ok, err := store.GetDelJSON(context.Background(), "key", &got)
	if err != nil {
		t.Fatalf("getdel json: %v", err)
	}
	if !ok || got["hello"] != "world" {
		t.Fatalf("unexpected value: ok=%v got=%#v", ok, got)
	}
	if srv.Exists("key") {
		t.Fatal("expected key to be deleted")
	}
}

// TestNewRequiredFailsForInvalidURL verifies API startup can fail fast when Redis is misconfigured.
func TestNewRequiredFailsForInvalidURL(t *testing.T) {
	if _, err := NewRequired(context.Background(), "://bad-url", time.Minute, nil); err == nil {
		t.Fatal("expected invalid redis URL to fail")
	}
}

// TestNewRequiredFailsWithoutURL verifies required Redis stores cannot be disabled silently.
func TestNewRequiredFailsWithoutURL(t *testing.T) {
	if _, err := NewRequired(context.Background(), "", time.Minute, nil); err == nil {
		t.Fatal("expected missing redis URL to fail")
	}
}

// TestNewRequiredFailsWhenRedisUnavailable verifies API startup can fail fast when Redis cannot be reached.
func TestNewRequiredFailsWhenRedisUnavailable(t *testing.T) {
	srv := miniredis.RunT(t)
	addr := srv.Addr()
	srv.Close()

	if _, err := NewRequired(context.Background(), "redis://"+addr, time.Minute, nil); err == nil {
		t.Fatal("expected unavailable redis to fail")
	}
}
