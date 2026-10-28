package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateSMTPConfigRequiresLeaseLongerThanSendTimeout(t *testing.T) {
	cfg := WorkerConfig{
		NotificationConfig: NotificationConfig{NotificationsEnabled: true},
		SMTPConfig: SMTPConfig{
			SMTPHost:          "smtp.example.com",
			SMTPPort:          587,
			SMTPTLSMode:       "starttls",
			SMTPTimeout:       time.Minute,
			MailPollInterval:  time.Second,
			MailBatchSize:     1,
			MailLeaseDuration: time.Minute,
			MailMaxAttempts:   3,
			MailRetryBase:     time.Second,
		},
	}

	err := validateSMTPConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "lease_duration") {
		t.Fatalf("expected lease validation error, got %v", err)
	}
}
