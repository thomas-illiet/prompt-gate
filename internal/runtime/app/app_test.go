package app

import (
	"context"
	"testing"
	"time"

	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
)

func TestStartSubscriptionWatcherSubscribesOnce(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store, err := redisstore.NewRequired(
		context.Background(),
		"redis://"+redisServer.Addr(),
		time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("create redis store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	application := &App{
		QuotaRedis: subscriptions.NewRedisStore(store, nil, time.Minute, nil),
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	application.StartSubscriptionWatcher(ctx)
	application.StartSubscriptionWatcher(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for redisServer.PubSubNumSub(redisstore.EventsChannel)[redisstore.EventsChannel] != 1 {
		if time.Now().After(deadline) {
			t.Fatalf(
				"expected exactly one subscription watcher, got %d",
				redisServer.PubSubNumSub(redisstore.EventsChannel)[redisstore.EventsChannel],
			)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
