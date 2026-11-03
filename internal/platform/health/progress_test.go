package health

import (
	"context"
	"testing"
	"time"
)

func TestProgressDetectsStalledJobEvenWhenOtherJobsRun(t *testing.T) {
	p := NewProgress()
	now := time.Now()
	p.now = func() time.Time { return now }
	ctx := WithProgress(context.Background(), p)
	Beat(ctx, "usage", time.Minute)
	Run(ctx, "daily", 24*time.Hour, func() {
		now = now.Add(9 * time.Minute)
		Beat(ctx, "usage", time.Minute)
		if !p.Healthy() {
			t.Fatal("running jobs should be healthy within their budgets")
		}
		now = now.Add(2 * time.Minute)
		Beat(ctx, "usage", time.Minute)
		if p.Healthy() {
			t.Fatal("another heartbeat masked a stuck job")
		}
	})
	if !p.Healthy() {
		t.Fatal("completed daily job must remain healthy during its idle interval")
	}
	now = now.Add(time.Minute)
	if p.Healthy() {
		t.Fatal("stalled usage consumer was not detected")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	Run(cancelled, "cancelled", time.Minute, func() { t.Fatal("cancelled job ran") })
}
