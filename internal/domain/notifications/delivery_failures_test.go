package notifications

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

type failingDeliverySender struct{ calls int }

func (s *failingDeliverySender) Send(context.Context, smtpEnvelope) error {
	s.calls++
	return errors.New(strings.Repeat("temporary SMTP failure ", 150))
}

func TestDeliveryRetriesBackOffAndStopAtMaximum(t *testing.T) {
	service, db := notificationTestService(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	ctx := context.Background()
	message, _, err := service.EnqueueManual(ctx, ManualMessageRequest{Subject: "Retry", Markdown: "Hello", Audience: AudienceRequest{Mode: "external", ExternalAddress: "ops@example.test"}}, nil, "retry-budget")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(time.Second)
	worker := NewWorker(db, WorkerOptions{BatchSize: 1, LeaseDuration: time.Minute, MaxAttempts: 2, RetryBase: time.Minute, Timeout: time.Second}, nil)
	sender := &failingDeliverySender{}
	worker.sender = sender
	worker.now = func() time.Time { return now }
	load := func() Delivery {
		t.Helper()
		var d Delivery
		if err := db.Where("message_id = ?", message.ID).Take(&d).Error; err != nil {
			t.Fatal(err)
		}
		return d
	}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	first := load()
	if first.Status != DeliveryPending || first.Attempts != 1 || first.LeaseUntil != nil || len(first.LastError) != 2000 || !first.NextAttemptAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("retry not scheduled: %#v", first)
	}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 {
		t.Fatal("retry ignored backoff")
	}
	now = now.Add(time.Minute)
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	failed := load()
	if failed.Status != DeliveryFailed || failed.Attempts != 2 || sender.calls != 2 {
		t.Fatalf("retry budget ignored: %#v", failed)
	}
	now = now.Add(24 * time.Hour)
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 2 {
		t.Fatal("terminal delivery retried automatically")
	}
	if worker.retryDelay(100) != 256*time.Minute || worker.retryDelay(0) != time.Minute {
		t.Fatal("unbounded or invalid retry delay")
	}
	if count, err := service.RetryFailed(ctx, message.ID, now); err != nil || count != 1 {
		t.Fatalf("manual retry: %d %v", count, err)
	}
	worker.sender = &fakeSender{}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if delivered := load(); delivered.Status != DeliverySent || delivered.SentAt == nil || delivered.LastError != "" {
		t.Fatalf("retry recovery: %#v", delivered)
	}
}

func TestNotificationWorkerDefaultsAndCancellation(t *testing.T) {
	_, db := notificationTestService(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	worker := NewWorker(db, WorkerOptions{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := worker.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if worker.opts.BatchSize != 25 || worker.opts.MaxAttempts != 8 || worker.opts.LeaseDuration != time.Minute {
		t.Fatal("invalid worker defaults")
	}
	if err := worker.runOnce(context.Background()); err != nil {
		t.Fatalf("empty queue: %v", err)
	}
	_ = sqlDB.Close()
	if err := worker.runOnce(context.Background()); err == nil {
		t.Fatal("claim failure hidden")
	}
}

func TestSMTPRejectsUnsupportedSecurityAndProtocolFailures(t *testing.T) {
	for _, tc := range []struct{ name, mode, username, reject, expected string }{
		{"greeting", "plain", "", "GREETING", "initialize smtp"},
		{"STARTTLS required", "starttls", "", "", "does not support STARTTLS"},
		{"authentication required", "plain", "operator", "", "does not support authentication"},
		{"sender rejected", "plain", "", "MAIL", "set smtp sender"},
		{"recipient rejected", "plain", "", "RCPT", "set smtp recipient"},
		{"DATA rejected", "plain", "", "DATA", "start smtp data"},
		{"body rejected", "plain", "", "BODY", "finish smtp data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				c, err := listener.Accept()
				if err != nil {
					return
				}
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(2 * time.Second))
				reader := bufio.NewReader(c)
				if tc.reject == "GREETING" {
					fmt.Fprint(c, "554 unavailable\r\n")
					return
				}
				fmt.Fprint(c, "220 localhost ready\r\n")
				data := false
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if data {
						if line == ".\r\n" {
							fmt.Fprint(c, "554 body rejected\r\n")
							return
						}
						continue
					}
					command := strings.Fields(line)[0]
					if command == tc.reject {
						fmt.Fprint(c, "550 rejected\r\n")
						return
					}
					if command == "DATA" {
						fmt.Fprint(c, "354 send body\r\n")
						data = true
					} else {
						fmt.Fprint(c, "250 OK\r\n")
					}
				}
			}()
			sender := networkSMTPSender{opts: WorkerOptions{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, TLSMode: tc.mode, Username: tc.username, Timeout: time.Second}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = sender.Send(ctx, smtpEnvelope{From: "from@example.test", To: "to@example.test", Subject: "Feature", TextBody: "Test", ReplyTo: "reply@example.test"})
			if err == nil || !strings.Contains(err.Error(), tc.expected) {
				t.Fatalf("protocol rejection lost: %v", err)
			}
			<-done
		})
	}
}
