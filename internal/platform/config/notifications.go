package config

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

func notificationConfig(file fileConfig) NotificationConfig {
	return NotificationConfig{
		NotificationsEnabled: file.Common.Notifications.Enabled,
		MailFromName:         strings.TrimSpace(file.Common.Notifications.FromName),
		MailFromAddress:      strings.TrimSpace(file.Common.Notifications.FromAddress),
		MailReplyToAddress:   strings.TrimSpace(file.Common.Notifications.ReplyTo),
	}
}

func notificationScheduleConfig(file fileConfig) NotificationScheduleConfig {
	return NotificationScheduleConfig{
		NotificationCleanupInterval: time.Duration(file.Schedule.NotificationCleanupInterval),
		NotificationRetention:       time.Duration(file.Schedule.NotificationRetention),
		QuotaNotificationInterval:   time.Duration(file.Schedule.QuotaNotificationInterval),
	}
}

func smtpConfig(file fileConfig) SMTPConfig {
	return SMTPConfig{
		SMTPHost:          strings.TrimSpace(file.Worker.SMTP.Host),
		SMTPPort:          file.Worker.SMTP.Port,
		SMTPUsername:      strings.TrimSpace(file.Worker.SMTP.Username),
		SMTPPassword:      file.Worker.SMTP.Password,
		SMTPTLSMode:       strings.ToLower(strings.TrimSpace(file.Worker.SMTP.TLSMode)),
		SMTPTimeout:       time.Duration(file.Worker.SMTP.Timeout),
		MailPollInterval:  time.Duration(file.Worker.SMTP.PollInterval),
		MailBatchSize:     file.Worker.SMTP.BatchSize,
		MailLeaseDuration: time.Duration(file.Worker.SMTP.LeaseDuration),
		MailMaxAttempts:   file.Worker.SMTP.MaxAttempts,
		MailRetryBase:     time.Duration(file.Worker.SMTP.RetryBase),
	}
}

func validateNotificationConfig(cfg NotificationConfig) error {
	if !cfg.NotificationsEnabled {
		return nil
	}
	if err := validateBareAddress(cfg.MailFromAddress); err != nil {
		return errors.New("common.notifications.from_address must be one valid email address without a display name")
	}
	if strings.ContainsAny(cfg.MailFromName, "\r\n") {
		return errors.New("common.notifications.from_name must not contain line breaks")
	}
	if cfg.MailReplyToAddress != "" {
		if err := validateBareAddress(cfg.MailReplyToAddress); err != nil {
			return errors.New("common.notifications.reply_to must be one valid email address without a display name")
		}
	}
	return nil
}

func validateBareAddress(value string) error {
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return errors.New("invalid address")
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value {
		return errors.New("invalid address")
	}
	return nil
}

func validateSMTPConfig(cfg WorkerConfig) error {
	if !cfg.NotificationsEnabled {
		return nil
	}
	if cfg.SMTPHost == "" {
		return errors.New("worker.smtp.host is required when notifications are enabled")
	}
	if cfg.SMTPPort <= 0 || cfg.SMTPPort > 65535 {
		return errors.New("worker.smtp.port must be between 1 and 65535")
	}
	switch cfg.SMTPTLSMode {
	case "plain", "starttls", "tls":
	default:
		return errors.New("worker.smtp.tls_mode must be plain, starttls, or tls")
	}
	if (cfg.SMTPUsername == "") != (cfg.SMTPPassword == "") {
		return errors.New("worker.smtp.username and worker.smtp.password must be configured together")
	}
	if cfg.MailBatchSize <= 0 {
		return errors.New("worker.smtp.batch_size must be greater than zero")
	}
	if cfg.MailMaxAttempts <= 0 {
		return errors.New("worker.smtp.max_attempts must be greater than zero")
	}
	if err := validatePositiveDurations(
		positiveDuration{"worker.smtp.timeout", cfg.SMTPTimeout},
		positiveDuration{"worker.smtp.poll_interval", cfg.MailPollInterval},
		positiveDuration{"worker.smtp.lease_duration", cfg.MailLeaseDuration},
		positiveDuration{"worker.smtp.retry_base", cfg.MailRetryBase},
	); err != nil {
		return err
	}
	if cfg.MailLeaseDuration <= cfg.SMTPTimeout {
		return errors.New("worker.smtp.lease_duration must be greater than worker.smtp.timeout")
	}
	return nil
}
