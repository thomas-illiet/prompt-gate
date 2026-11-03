package notifications

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestRegressionCleanupMustPreservePendingDelivery(t *testing.T) {
	s, db := notificationTestService(t)
	now := time.Now().UTC()
	m := Message{Kind: KindManual, Subject: "Pending audit message", CreatedAt: now.Add(-48 * time.Hour)}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	d := Delivery{MessageID: m.ID, RecipientEmail: "audit@example.com", Status: DeliveryPending}
	if err := db.Create(&d).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Cleanup(context.Background(), 24*time.Hour, now); err != nil {
		t.Fatal(err)
	}
	var remaining int64
	if err := db.Model(&Delivery{}).Where("id = ?", d.ID).Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatal("retention deleted an unsent pending notification")
	}
}
func TestRegressionMonthlyEmbeddingCostMustMatchDashboard(t *testing.T) {
	s, db := notificationTestService(t)
	s.SetUsageCostEstimator(regressionCostEstimator{})
	userID := "44444444-4444-4444-4444-444444444444"
	if err := db.Create(&testUser{ID: userID, Email: "audit@example.com", Name: "Audit", PreferredUsername: "audit", Type: "user", Role: "user", IsActive: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&testDailyKPI{Day: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), InitiatorID: userID, Requests: 1, InputTokens: 1000000, EmbeddingTokens: 1000000, TotalTokens: 1000000}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.GenerateMonthlyReports(context.Background(), time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC), time.UTC); err != nil {
		t.Fatal(err)
	}
	var m Message
	if err := db.Where("kind = ?", KindMonthlyReport).Take(&m).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.MarkdownBody, "$0.0200 USD") {
		t.Fatalf("embedding-only monthly report must cost $0.0200, body: %s", m.MarkdownBody)
	}
}
func TestRegressionSMTPAcceptedMessageMustNotRetryOnQuitFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan bool, 1)
	go func() {
		c, err := listener.Accept()
		if err != nil {
			accepted <- false
			return
		}
		defer c.Close()
		_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		reader := bufio.NewReader(c)
		fmt.Fprint(c, "220 localhost ready\r\n")
		data := false
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				accepted <- false
				return
			}
			if data {
				if line == ".\r\n" {
					data = false
					fmt.Fprint(c, "250 message accepted\r\n")
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"), strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
				fmt.Fprint(c, "250 OK\r\n")
			case strings.HasPrefix(line, "DATA"):
				data = true
				fmt.Fprint(c, "354 send body\r\n")
			case strings.HasPrefix(line, "QUIT"):
				accepted <- true
				return
			default:
				fmt.Fprint(c, "500 unsupported\r\n")
			}
		}
	}()
	sender := networkSMTPSender{opts: WorkerOptions{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Timeout: 3 * time.Second}}
	err = sender.Send(context.Background(), smtpEnvelope{From: "from@example.com", To: "to@example.com", MessageID: "audit@localhost", Subject: "Audit", TextBody: "Local test only"})
	if !<-accepted {
		t.Fatal("local SMTP fixture did not accept message")
	}
	if err != nil {
		t.Fatalf("SMTP accepted message but sender reports retryable failure: %v", err)
	}
}

type regressionCostEstimator struct{}

func (regressionCostEstimator) EstimateReportCost(_ context.Context, _ string, start, end time.Time) (*float64, error) {
	if !start.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) || !end.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)) {
		return nil, fmt.Errorf("unexpected report period: %s - %s", start, end)
	}
	cost := 0.02
	return &cost, nil
}

type cancellingSender struct{ cancel context.CancelFunc }

func (s cancellingSender) Send(context.Context, smtpEnvelope) error { s.cancel(); return nil }
func TestAcceptedMailIsRecordedDuringShutdown(t *testing.T) {
	_, db := notificationTestService(t)
	now := time.Now().UTC()
	message := Message{Kind: KindManual, Subject: "Shutdown", CreatedAt: now}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	delivery := Delivery{MessageID: message.ID, RecipientEmail: "test@example.com", Status: DeliveryPending, NextAttemptAt: now}
	if err := db.Create(&delivery).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := NewWorker(db, WorkerOptions{BatchSize: 1, LeaseDuration: time.Minute}, nil)
	worker.sender = cancellingSender{cancel}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&delivery, "id = ?", delivery.ID).Error; err != nil {
		t.Fatal(err)
	}
	if delivery.Status != DeliverySent || delivery.SentAt == nil {
		t.Fatalf("accepted mail left retryable: %#v", delivery)
	}
}
func TestCleanupRetainsEveryUnfinishedOrRecentlySentMessage(t *testing.T) {
	for _, status := range []DeliveryStatus{DeliveryPending, DeliveryProcessing, DeliveryFailed, DeliverySent} {
		t.Run(string(status), func(t *testing.T) {
			s, db := notificationTestService(t)
			now := time.Now().UTC()
			message := Message{Kind: KindManual, Subject: string(status), CreatedAt: now.Add(-48 * time.Hour)}
			if err := db.Create(&message).Error; err != nil {
				t.Fatal(err)
			}
			delivery := Delivery{MessageID: message.ID, RecipientEmail: "test@example.com", Status: status}
			if status == DeliverySent {
				delivery.SentAt = &now
			}
			if err := db.Create(&delivery).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := s.Cleanup(context.Background(), 24*time.Hour, now); err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := db.Model(&Delivery{}).Where("id = ?", delivery.ID).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("removed %s delivery: %v", status, err)
			}
		})
	}
}
