package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type testUser struct {
	ID                string `gorm:"primaryKey"`
	Email             string
	Name              string
	PreferredUsername string
	Type              string
	Role              string
	IsActive          bool
	ExpiresAt         *time.Time
}

func (testUser) TableName() string { return "users" }

type testGroupMember struct {
	GroupID string
	UserID  string
}

func (testGroupMember) TableName() string { return "access_group_members" }

type testQuotaProjection struct {
	UserID        string `gorm:"primaryKey"`
	Used5HTokens  int64  `gorm:"column:used_5h_tokens"`
	Quota5HTokens *int64 `gorm:"column:quota_5h_tokens"`
	Used7DTokens  int64  `gorm:"column:used_7d_tokens"`
	Quota7DTokens *int64 `gorm:"column:quota_7d_tokens"`
}

func (testQuotaProjection) TableName() string { return "subscription_quota_states" }

type testDailyKPI struct {
	Day                   time.Time `gorm:"primaryKey"`
	InitiatorID           string    `gorm:"primaryKey"`
	Requests              int64
	TotalDurationMs       int64
	InputTokens           int64
	OutputTokens          int64
	CacheReadInputTokens  int64
	CacheWriteInputTokens int64
	EmbeddingTokens       int64
	TotalTokens           int64
}

func (testDailyKPI) TableName() string { return "proxy_daily_usage_kpis" }

type testBreakdown struct {
	ID          string `gorm:"primaryKey"`
	Day         time.Time
	InitiatorID string
	Dimension   string
	Name        string
	TotalTokens int64
}

func (testBreakdown) TableName() string { return "proxy_daily_usage_breakdowns" }

func notificationTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&testUser{}, &testGroupMember{}, &testQuotaProjection{}, &testDailyKPI{}, &testBreakdown{}); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, true, "https://promptgate.example")
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return service, db
}

func TestManualNotificationLifecycle(t *testing.T) {
	service, db := notificationTestService(t)
	users := []testUser{
		{ID: "11111111-1111-1111-1111-111111111111", Email: "alice@example.com", Name: "Alice", PreferredUsername: "alice", Type: "user", Role: "user", IsActive: true},
		{ID: "22222222-2222-2222-2222-222222222222", Email: "invalid", Name: "Bad", PreferredUsername: "bad", Type: "user", Role: "none"},
		{ID: "33333333-3333-3333-3333-333333333333", Email: "service@example.com", Type: "service", Role: "user", IsActive: true},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	audience, err := service.ResolveAudience(context.Background(), AudienceRequest{Mode: "all"})
	if err != nil || audience.Valid != 1 || audience.Ignored != 1 {
		t.Fatalf("unexpected audience: %#v err=%v", audience, err)
	}
	previewText, previewHTML, err := service.Preview("Status update", "# Hello\n\n<script>alert(1)</script> **world**")
	if err != nil || !strings.Contains(previewText, "Hello") || strings.Contains(previewHTML, "script") || !strings.Contains(previewHTML, "/email/manual.jpg") {
		t.Fatalf("unsafe preview: %q %q %v", previewText, previewHTML, err)
	}
	request := ManualMessageRequest{Subject: "Status update", Markdown: "# Hello\n\nEverything is ready.", Audience: AudienceRequest{Mode: "all"}}
	first, _, err := service.EnqueueManual(context.Background(), request, nil, "request-1")
	if err != nil || first.Recipients != 1 {
		t.Fatalf("enqueue: %#v %v", first, err)
	}
	replayed, _, err := service.EnqueueManual(context.Background(), request, nil, "request-1")
	if err != nil || replayed.ID != first.ID {
		t.Fatalf("replay created another message: %#v %v", replayed, err)
	}
	list, err := service.ListMessages(context.Background(), ListParams{Page: 1, PageSize: 10, SortDir: "desc"})
	if err != nil || list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("list: %#v %v", list, err)
	}
	if err := db.Model(&Delivery{}).Where("message_id = ?", first.ID).Updates(map[string]any{"status": DeliveryFailed, "last_error": "temporary"}).Error; err != nil {
		t.Fatal(err)
	}
	failed, err := service.ListMessages(context.Background(), ListParams{Page: 1, PageSize: 10, Status: DeliveryFailed, SortDir: "desc"})
	if err != nil || failed.Total != 1 || len(failed.Items) != 1 {
		t.Fatalf("filter failed messages: %#v %v", failed, err)
	}
	count, err := service.RetryFailed(context.Background(), first.ID, time.Now())
	if err != nil || count != 1 {
		t.Fatalf("retry: %d %v", count, err)
	}
	if _, err := service.GetMessage(context.Background(), "missing"); !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := db.Model(&Message{}).Where("id = ?", first.ID).Update("created_at", time.Now().Add(-48*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	deleted, err := service.Cleanup(context.Background(), 24*time.Hour, time.Now())
	if err != nil || deleted != 1 {
		t.Fatalf("cleanup: %d %v", deleted, err)
	}
	var remainingDeliveries int64
	if err := db.Model(&Delivery{}).Where("message_id = ?", first.ID).Count(&remainingDeliveries).Error; err != nil || remainingDeliveries != 0 {
		t.Fatalf("cleanup left %d deliveries: %v", remainingDeliveries, err)
	}
}

func TestRendererUsesDistinctBrandedTemplates(t *testing.T) {
	t.Parallel()
	renderer := NewRenderer()
	tests := []struct {
		kind  Kind
		image string
		title string
	}{
		{KindWelcome, "welcome.jpg", "Welcome, Ada. Everything is ready."},
		{KindQuotaExhausted, "quota-exhausted.jpg", "Your quota is taking a little nap."},
		{KindQuotaReset, "quota-reset-granted.jpg", "A reset just landed."},
		{KindGroupChanged, "group-changed.jpg", "New team, new doors."},
		{KindTokenCreated, "token-created.jpg", "A brand-new key. Keep it safe."},
		{KindTokenExpired, "token-expired.jpg", "This key has finished its shift."},
		{KindTokenRevoked, "token-revoked.jpg", "This key is not getting through."},
		{KindMonthlyReport, "monthly-usage-report.jpg", "Your month, by the numbers — but fun."},
		{KindManual, "manual.jpg", "Maintenance complete"},
	}
	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			textBody, htmlBody, err := renderer.RenderTemplate(test.kind, "Ada", "Maintenance complete", "# Duplicate heading\n\nUseful **details**.", "https://promptgate.example")
			if err != nil {
				t.Fatal(err)
			}
			supporting := templateFor(test.kind, "Ada", "Maintenance complete").supporting
			if !strings.Contains(htmlBody, test.title) || !strings.Contains(htmlBody, supporting) || !strings.Contains(htmlBody, "/email/"+test.image) || !strings.Contains(htmlBody, "Useful <strong>details</strong>") || !strings.Contains(htmlBody, `width="620" height="310"`) || !strings.Contains(htmlBody, "aspect-ratio:2/1") {
				t.Fatalf("template %s missing expected content: %s", test.kind, htmlBody)
			}
			if strings.Contains(htmlBody, "<script") || !strings.Contains(textBody, supporting) || !strings.Contains(textBody, "https://promptgate.example") {
				t.Fatalf("template %s produced unsafe or incomplete alternatives", test.kind)
			}
		})
	}
}

func TestQuotaTransitionAndMonthlyReport(t *testing.T) {
	service, db := notificationTestService(t)
	service.SetUsageCostRates(5, 30, 0.02)
	userID := "44444444-4444-4444-4444-444444444444"
	if err := db.Create(&testUser{ID: userID, Email: "person@example.com", Name: "Person", PreferredUsername: "person", Type: "user", Role: "user", IsActive: true}).Error; err != nil {
		t.Fatal(err)
	}
	quota := int64(100)
	projection := testQuotaProjection{UserID: userID, Used5HTokens: 100, Quota5HTokens: &quota, Used7DTokens: 20, Quota7DTokens: &quota}
	if err := db.Create(&projection).Error; err != nil {
		t.Fatal(err)
	}
	if emitted, err := service.ScanQuotaExhaustion(context.Background(), time.Now()); err != nil || emitted != 0 {
		t.Fatalf("initial scan: %d %v", emitted, err)
	}
	if err := db.Model(&testQuotaProjection{}).Where("user_id = ?", userID).Updates(map[string]any{"used_5h_tokens": 50, "used_7d_tokens": 100}).Error; err != nil {
		t.Fatal(err)
	}
	if emitted, err := service.ScanQuotaExhaustion(context.Background(), time.Now()); err != nil || emitted != 1 {
		t.Fatalf("transition scan: %d %v", emitted, err)
	}

	day := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	if err := db.Create(&testDailyKPI{Day: day, InitiatorID: userID, Requests: 2, TotalDurationMs: 300, InputTokens: 20, OutputTokens: 10, TotalTokens: 30}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&testBreakdown{ID: "breakdown-1", Day: day, InitiatorID: userID, Dimension: "models", Name: "gpt-test", TotalTokens: 30}).Error; err != nil {
		t.Fatal(err)
	}
	paris, _ := time.LoadLocation("Europe/Paris")
	// A scheduler outage on the first day must not lose the monthly report.
	now := time.Date(2026, 10, 2, 8, 30, 0, 0, paris)
	created, err := service.GenerateMonthlyReports(context.Background(), now, paris)
	if err != nil || created != 1 {
		t.Fatalf("monthly report: %d %v", created, err)
	}
	var report Message
	if err := db.Where("kind = ?", KindMonthlyReport).Take(&report).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.MarkdownBody, "gpt-test") || !strings.Contains(report.MarkdownBody, "**Requests:** 2") {
		t.Fatalf("unexpected report: %s", report.MarkdownBody)
	}
}

type fakeSender struct {
	err  error
	sent []smtpEnvelope
}

func (f *fakeSender) Send(_ context.Context, envelope smtpEnvelope) error {
	f.sent = append(f.sent, envelope)
	return f.err
}

func TestWorkerSendsAndRetries(t *testing.T) {
	service, db := notificationTestService(t)
	if err := db.Create(&testUser{ID: "55555555-5555-5555-5555-555555555555", Email: "worker@example.com", Name: "Worker", PreferredUsername: "worker", Type: "user", Role: "user", IsActive: true}).Error; err != nil {
		t.Fatal(err)
	}
	message, _, err := service.EnqueueManual(context.Background(), ManualMessageRequest{Subject: "Worker", Markdown: "Hello", Audience: AudienceRequest{Mode: "all"}}, nil, "worker-send")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	sender := &fakeSender{}
	worker := NewWorker(db, WorkerOptions{FromAddress: "from@example.com", Host: "localhost", Port: 25, BatchSize: 5, LeaseDuration: time.Minute, MaxAttempts: 2, RetryBase: time.Second}, nil)
	worker.sender, worker.now = sender, func() time.Time { return now }
	if err := worker.runOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetMessage(context.Background(), message.ID)
	if err != nil || detail.Sent != 1 || len(sender.sent) != 1 {
		t.Fatalf("send state: %#v sent=%d err=%v", detail, len(sender.sent), err)
	}
	if payload := string(buildMIMEMessage(sender.sent[0])); !strings.Contains(payload, "multipart/alternative") || !strings.Contains(payload, "Message-ID") {
		t.Fatalf("invalid MIME: %s", payload)
	}
}

func TestWorkerSkipsAClaimReclaimedByAnotherWorker(t *testing.T) {
	service, db := notificationTestService(t)
	if err := db.Create(&testUser{ID: "66666666-6666-6666-6666-666666666666", Email: "lease@example.com", Name: "Lease", PreferredUsername: "lease", Type: "user", Role: "user", IsActive: true}).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.EnqueueManual(context.Background(), ManualMessageRequest{Subject: "Lease", Markdown: "Hello", Audience: AudienceRequest{Mode: "all"}}, nil, "worker-lease"); err != nil {
		t.Fatal(err)
	}

	var queued Delivery
	if err := db.Where("recipient_email = ?", "lease@example.com").Take(&queued).Error; err != nil {
		t.Fatalf("load queued delivery: %v", err)
	}
	start := queued.NextAttemptAt.Add(time.Second)
	first := NewWorker(db, WorkerOptions{BatchSize: 1, LeaseDuration: time.Minute}, nil)
	first.now = func() time.Time { return start }
	firstClaim, err := first.claim(context.Background(), start)
	if err != nil || len(firstClaim) != 1 {
		t.Fatalf("first claim: %#v %v", firstClaim, err)
	}

	second := NewWorker(db, WorkerOptions{BatchSize: 1, LeaseDuration: time.Minute}, nil)
	reclaimedAt := start.Add(2 * time.Minute)
	second.now = func() time.Time { return reclaimedAt }
	secondClaim, err := second.claim(context.Background(), reclaimedAt)
	if err != nil || len(secondClaim) != 1 {
		t.Fatalf("second claim: %#v %v", secondClaim, err)
	}
	if owned, err := first.renewClaim(context.Background(), firstClaim[0]); err != nil || owned {
		t.Fatalf("stale worker renewed reclaimed delivery: owned=%t err=%v", owned, err)
	}
	if owned, err := second.renewClaim(context.Background(), secondClaim[0]); err != nil || !owned {
		t.Fatalf("current worker could not renew delivery: owned=%t err=%v", owned, err)
	}
}

func TestBuildMIMEMessageSanitizesDisplayNames(t *testing.T) {
	payload := string(buildMIMEMessage(smtpEnvelope{
		MessageID: "delivery@promptgate",
		FromName:  "Prompt Gate\r\nBcc: attacker@example.com",
		From:      "from@example.com",
		ToName:    "Recipient\nCc: attacker@example.com",
		To:        "recipient@example.com",
		Subject:   "Safe subject",
		TextBody:  "Hello",
		HTMLBody:  "<p>Hello</p>",
	}))
	if strings.Contains(payload, "\r\nBcc:") || strings.Contains(payload, "\r\nCc:") {
		t.Fatalf("display name injected a MIME header: %s", payload)
	}
}
