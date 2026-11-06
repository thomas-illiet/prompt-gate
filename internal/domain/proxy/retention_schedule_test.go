package proxy

import (
	"context"
	"testing"
	"time"
)

func TestScheduledRetentionPreservesTotalsAndPendingReplayMarkers(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userID := "11111111-1111-1111-1111-111111111111"
	old := time.Now().UTC().Add(-72 * time.Hour)
	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "model", old, 10, 20)
	mustAggregateUsageKPIs(t, service)
	for _, id := range []string{"processed", "pending"} {
		if err := db.Create(&ProcessedUsageEvent{EventID: id, ProcessedAt: old}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&PendingUsageEvent{EventID: "pending", Payload: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	service.StartRawUsageCleanup(ctx, 24*time.Hour, 10*time.Millisecond)
	deadline := time.Now().Add(time.Second)
	for {
		var raw, markers int64
		if err := db.Model(&Interception{}).Count(&raw).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&ProcessedUsageEvent{}).Where("event_id = ?", "processed").Count(&markers).Error; err != nil {
			t.Fatal(err)
		}
		if raw == 0 && markers == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("scheduled retention did not run")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	var pending int64
	if err := db.Model(&ProcessedUsageEvent{}).Where("event_id = ?", "pending").Count(&pending).Error; err != nil || pending != 1 {
		t.Fatalf("retry deduplication marker removed: %d %v", pending, err)
	}
	var total int64
	if err := db.Table("proxy_daily_usage_kpis").Select("SUM(total_tokens)").Scan(&total).Error; err != nil || total != 37 {
		t.Fatalf("retention changed durable totals: %d %v", total, err)
	}
}
