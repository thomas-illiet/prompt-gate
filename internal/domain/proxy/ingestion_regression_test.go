package proxy

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/requestprivacy"

	"github.com/alicebob/miniredis/v2"
	aibrecorder "github.com/coder/aibridge/recorder"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const regressionUserID = "11111111-1111-1111-1111-111111111111"
const regressionInterceptionID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

func regressionRedis(t *testing.T) (*miniredis.Miniredis, *redisstore.Store) {
	t.Helper()
	server := miniredis.RunT(t)
	store, err := redisstore.NewRequired(context.Background(), "redis://"+server.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return server, store
}

func regressionEvents(at time.Time) []UsageEvent {
	return []UsageEvent{
		{EventID: "start", Type: UsageEventInterceptionStarted, CreatedAt: at, InterceptionStarted: &InterceptionStartedEvent{
			ID: regressionInterceptionID, InitiatorID: regressionUserID, Provider: "provider", ProviderType: "anthropic", Model: "model", ClientIP: "198.51.100.8", StartedAt: at,
		}},
		{EventID: "tokens", Type: UsageEventTokenUsage, CreatedAt: at.Add(time.Second), TokenUsage: &TokenUsageEvent{
			InterceptionID: regressionInterceptionID, ProviderResponseID: "message", InputTokens: 100, OutputTokens: 50, CacheReadInputTokens: 60, CacheWriteInputTokens: 20, CreatedAt: at,
		}},
	}
}

func processRegressionEvents(t *testing.T, worker *Worker, events []UsageEvent) {
	t.Helper()
	for _, event := range events {
		if err := worker.ProcessUsageEvent(context.Background(), event, event.EventID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAccountTotalsMatchDashboardAfterRetention(t *testing.T) {
	for _, accountType := range []auth.UserType{auth.UserTypeUser, auth.UserTypeService} {
		t.Run(string(accountType), func(t *testing.T) {
			db, dashboard := newProxyServiceTestDB(t)
			ctx := context.Background()
			if err := db.Model(&users.User{}).Where("id = ?", regressionUserID).Update("type", accountType).Error; err != nil {
				t.Fatal(err)
			}
			accounts := users.NewService(db)
			now := time.Now().UTC()
			// Another identity has more non-cached input but fewer total tokens.
			// Pagination must follow the requested total, including cached tokens.
			otherID := "22222222-2222-2222-2222-222222222222"
			if err := db.Model(&users.User{}).Where("id = ?", otherID).Update("type", accountType).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&ProxyDailyUsageKPI{Day: dayStart(now), InitiatorID: otherID, InputTokens: 200, TotalTokens: 200}).Error; err != nil {
				t.Fatal(err)
			}
			processRegressionEvents(t, NewWorker(db, nil, WorkerOptions{}, nil), regressionEvents(now.AddDate(0, 0, -100)))
			for _, cleanup := range []bool{false, true} {
				if cleanup {
					if n, err := dashboard.DeleteRawUsageBefore(ctx, now.AddDate(0, 0, -90)); err != nil || n != 1 {
						t.Fatalf("cleanup=%d, %v", n, err)
					}
				}
				stats, err := dashboard.DashboardOverview(ctx, regressionUserID, UsageWindowAll, now)
				if err != nil {
					t.Fatal(err)
				}
				var input, output, read, write, total int64
				if accountType == auth.UserTypeUser {
					list, err := accounts.ListUsers(ctx, users.ListParams{SortBy: "totalTokens", SortDir: "desc", PageSize: 1})
					if err != nil {
						t.Fatal(err)
					}
					if len(list.Items) != 1 || list.Items[0].ID != regressionUserID {
						t.Fatalf("wrong sorted user: %#v", list)
					}
					item := list.Items[0]
					input, output, read, write, total = item.InputTokens, item.OutputTokens, item.CacheReadInputTokens, item.CacheWriteInputTokens, item.TotalTokens
				} else {
					list, err := accounts.ListServiceAccountsPaged(ctx, users.ServiceAccountListParams{SortBy: "totalTokens", SortDir: "desc", PageSize: 1})
					if err != nil {
						t.Fatal(err)
					}
					if len(list.Items) != 1 || list.Items[0].ID != regressionUserID {
						t.Fatalf("wrong sorted account: %#v", list)
					}
					item := list.Items[0]
					input, output, read, write, total = item.InputTokens, item.OutputTokens, item.CacheReadInputTokens, item.CacheWriteInputTokens, item.TotalTokens
				}
				if input != 100 || output != 50 || read != 60 || write != 20 || total != 230 || total != stats.Totals.TotalTokens {
					t.Fatalf("cleanup=%t account=%d/%d/%d/%d total=%d dashboard=%d", cleanup, input, output, read, write, total, stats.Totals.TotalTokens)
				}
			}
		})
	}
}

func TestQuotaAndKPIShareNormalizedUsageAcrossPlanChanges(t *testing.T) {
	for _, private := range []bool{false, true} {
		t.Run(map[bool]string{false: "external", true: "model_lab"}[private], func(t *testing.T) {
			db, dashboard := newProxyServiceTestDB(t)
			_, redisStore := regressionRedis(t)
			ctx := clientip.ContextWithClientIP(context.Background(), "198.51.100.8")
			if private {
				ctx = requestprivacy.MarkMetadataOnly(ctx)
			}
			plans := subscriptions.NewService(db)
			if err := plans.AutoMigrate(ctx); err != nil {
				t.Fatal(err)
			}
			plans.SetNotifier(redisStore)
			plan, err := plans.CreatePlan(ctx, subscriptions.PlanInput{Name: "Plan", IsDefault: true})
			if err != nil {
				t.Fatal(err)
			}
			quotas := subscriptions.NewRedisStore(redisStore, plans, time.Minute, nil)
			quotas.SyncVersion(ctx)
			if err := quotas.WarmSnapshot(ctx); err != nil {
				t.Fatal(err)
			}
			recorder := subscriptions.NewQuotaRecorder(NewPrivacyRecorder(NewRedisRecorder(redisStore, nil).WithDurableFallback(db)), quotas, nil)
			if err := recorder.RecordInterception(ctx, &aibrecorder.InterceptionRecord{ID: regressionInterceptionID, InitiatorID: regressionUserID, Provider: "anthropic", Model: "model"}); err != nil {
				t.Fatal(err)
			}
			// Cumulative callbacks may be duplicated or arrive out of order.
			for _, usage := range []aibrecorder.TokenUsageRecord{
				{MsgID: "one", Output: 240},
				{MsgID: "one", Input: 100, Output: 1, CacheReadInputTokens: 60, CacheWriteInputTokens: 20},
				{MsgID: "one", Output: 240},
				{MsgID: "two", Input: 50, Output: 10},
			} {
				usage.InterceptionID = regressionInterceptionID
				before := usage
				if err := recorder.RecordTokenUsage(ctx, &usage); err != nil {
					t.Fatal(err)
				}
				if usage.Input != before.Input || usage.Output != before.Output {
					t.Fatal("recorder modified caller usage")
				}
			}
			limit := int64(1000)
			if _, err := plans.UpdatePlan(ctx, plan.ID, subscriptions.PlanInput{Name: "Updated", IsDefault: true, Quota5HTokens: &limit, Quota7DTokens: &limit}); err != nil {
				t.Fatal(err)
			}
			// Maintenance must use the changed plan even before its watcher runs.
			if _, err := quotas.SyncQuotaStates(ctx, plans); err != nil {
				t.Fatal(err)
			}
			var projection subscriptions.SubscriptionQuotaState
			if err := db.First(&projection, "user_id = ?", regressionUserID).Error; err != nil {
				t.Fatal(err)
			}
			if projection.PlanName != "Updated" || projection.Used5HTokens != 480 || projection.Quota5HTokens == nil || *projection.Quota5HTokens != limit {
				t.Fatalf("projection=%#v", projection)
			}
			quota, err := quotas.CurrentQuota(ctx, regressionUserID, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if quota.Plan.Name != "Updated" || quota.Used5HTokens != 480 || quota.Used7DTokens != 480 {
				t.Fatalf("quota=%#v", quota)
			}
			worker := NewWorker(db, redisStore, WorkerOptions{}, nil)
			if err := ensureUsageConsumerGroup(ctx, redisStore.Client()); err != nil {
				t.Fatal(err)
			}
			streams, err := redisStore.Client().XReadGroup(ctx, &redis.XReadGroupArgs{Group: UsageEventsConsumerGroup, Consumer: "test", Streams: []string{UsageEventsStream, ">"}, Count: 100}).Result()
			if err != nil {
				t.Fatal(err)
			}
			worker.handleMessages(ctx, redisStore.Client(), streams[0].Messages)
			worker.handleMessages(ctx, redisStore.Client(), streams[0].Messages) // replay after ambiguous ACK
			stats, err := dashboard.DashboardOverview(ctx, regressionUserID, UsageWindowAll, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if stats.Totals.TotalTokens != 480 || stats.Totals.Requests != 1 {
				t.Fatalf("stats=%#v", stats.Totals)
			}
			if n := redisStore.Client().XLen(ctx, UsageEventsStream).Val(); n != 0 {
				t.Fatalf("events still pending: %d", n)
			}
			if private {
				var ips int64
				db.Model(&AccountIPAddress{}).Count(&ips)
				if ips != 0 {
					t.Fatal("Model Lab IP persisted")
				}
			}
		})
	}
}

func TestUsageFallbackSurvivesCancellationAndAmbiguousPublication(t *testing.T) {
	db, dashboard := newProxyServiceTestDB(t)
	_, store := regressionRedis(t)
	worker := NewWorker(db, store, WorkerOptions{BatchSize: 1}, nil)
	recorder := NewRedisRecorder(store, nil).WithDurableFallback(db)
	ctx := context.Background()
	events := regressionEvents(time.Now().Add(-time.Minute))
	// The start exists in Redis, but the producer cannot tell whether XADD
	// succeeded. Retrying through SQL must still account for it exactly once.
	if err := recorder.enqueue(ctx, events[0]); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, event := range []UsageEvent{events[1], events[0]} {
		if err := recorder.enqueue(cancelled, event); err != nil {
			t.Fatal(err)
		}
	}
	for range 4 {
		if err := worker.processFallbackBatch(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := worker.ProcessUsageEvent(ctx, events[0], "ambiguous-redis-id"); err != errUsageEventAlreadyProcessed {
		t.Fatalf("duplicate result: %v", err)
	}
	stats, err := dashboard.DashboardOverview(ctx, regressionUserID, UsageWindowAll, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Totals.Requests != 1 || stats.Totals.TotalTokens != 230 {
		t.Fatalf("stats=%#v", stats.Totals)
	}
	var remaining int64
	db.Model(&PendingUsageEvent{}).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("fallback rows=%d", remaining)
	}
	if err := db.Migrator().DropTable(&PendingUsageEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := recorder.enqueue(cancelled, events[0]); err == nil {
		t.Fatal("lost event reported as successful")
	}
}

func TestFallbackOrphanDoesNotStarveLaterEvents(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	recorder := NewRedisRecorder(nil, nil).WithDurableFallback(db)
	worker := NewWorker(db, nil, WorkerOptions{BatchSize: 1}, nil)
	ctx := context.Background()
	events := regressionEvents(time.Now())
	orphan := events[1]
	orphan.EventID = "orphan"
	orphan.TokenUsage = &TokenUsageEvent{InterceptionID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", InputTokens: 1}
	orphan.CreatedAt = time.Now().Add(-time.Hour)
	for _, event := range append([]UsageEvent{orphan}, events...) {
		if err := recorder.enqueue(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	for range 5 {
		if err := worker.processFallbackBatch(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var pending []PendingUsageEvent
	if err := db.Find(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].EventID != "orphan" {
		t.Fatalf("pending=%#v", pending)
	}
}

func TestRetentionKeepsDeduplicationWhileFallbackAwaitsReplay(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	ctx := context.Background()
	events := regressionEvents(time.Now().Add(-time.Hour))
	worker := NewWorker(db, nil, WorkerOptions{}, nil)
	processRegressionEvents(t, worker, events)
	// Redis processing committed, but the ambiguous publication also left a
	// fallback copy. Cleanup must preserve its marker until that copy is drained.
	if err := NewRedisRecorder(nil, nil).WithDurableFallback(db).enqueue(ctx, events[1]); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().Add(time.Hour)
	if n, err := service.DeleteProcessedUsageBefore(ctx, cutoff); err != nil || n != 1 {
		t.Fatalf("marker cleanup=%d, %v", n, err)
	}
	if err := worker.processFallbackBatch(ctx); err != nil {
		t.Fatal(err)
	}
	stats, err := service.DashboardOverview(ctx, regressionUserID, UsageWindowAll, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Totals.TotalTokens != 230 {
		t.Fatalf("replayed usage counted twice: %#v", stats.Totals)
	}
	if n, err := service.DeleteProcessedUsageBefore(ctx, cutoff); err != nil || n != 1 {
		t.Fatalf("marker cleanup after replay=%d, %v", n, err)
	}
}

func TestWorkerDrainsDurableFallbackDuringRedisOutage(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	server, store := regressionRedis(t)
	server.SetError("ERR simulated Redis outage")
	recorder := NewRedisRecorder(store, nil).WithDurableFallback(db)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, event := range regressionEvents(time.Now()) {
		if err := recorder.enqueue(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	worker := NewWorker(db, store, WorkerOptions{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("worker did not stop")
		}
	})
	waitForUsage(t, func() bool {
		var row ProxyDailyUsageKPI
		return db.Where("initiator_id = ?", regressionUserID).Take(&row).Error == nil && row.TotalTokens == 230
	})
}

func TestWorkerRecoversDeletedRedisGroupAndStream(t *testing.T) {
	for _, removeStream := range []bool{false, true} {
		t.Run(map[bool]string{false: "group", true: "stream"}[removeStream], func(t *testing.T) {
			db, _ := newProxyServiceTestDB(t)
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			sqlDB.SetMaxOpenConns(1)
			_, store := regressionRedis(t)
			ctx, cancel := context.WithCancel(context.Background())
			worker := NewWorker(db, store, WorkerOptions{BlockTimeout: 10 * time.Millisecond, PendingIdleTimeout: time.Millisecond}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			done := make(chan error, 1)
			go func() { done <- worker.Run(ctx) }()
			t.Cleanup(func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(3 * time.Second):
					t.Error("worker did not stop")
				}
			})
			waitForUsage(t, func() bool { return store.Client().XInfoGroups(ctx, UsageEventsStream).Err() == nil })
			if removeStream {
				err = store.Client().Del(ctx, UsageEventsStream).Err()
			} else {
				err = store.Client().XGroupDestroy(ctx, UsageEventsStream, UsageEventsConsumerGroup).Err()
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, event := range regressionEvents(time.Now()) {
				payload, err := json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				if err := store.Client().XAdd(ctx, &redis.XAddArgs{Stream: UsageEventsStream, Values: map[string]any{UsageEventPayloadField: string(payload)}}).Err(); err != nil {
					t.Fatal(err)
				}
			}
			waitForUsage(t, func() bool {
				var row ProxyDailyUsageKPI
				return db.Where("initiator_id = ?", regressionUserID).Take(&row).Error == nil && row.TotalTokens == 230
			})
		})
	}
}

func waitForUsage(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for usage ingestion")
}

func TestMalformedNonemptyIPStillRejectsEvent(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	event := regressionEvents(time.Now())[0]
	event.InterceptionStarted.ClientIP = "invalid"
	err := NewWorker(db, nil, WorkerOptions{}, nil).ProcessUsageEvent(context.Background(), event, "bad-ip")
	if err == nil || !strings.Contains(err.Error(), "invalid IP") {
		t.Fatalf("error=%v", err)
	}
	if err := db.First(&Interception{}, "id = ?", regressionInterceptionID).Error; err != gorm.ErrRecordNotFound {
		t.Fatalf("interception persisted: %v", err)
	}
}
