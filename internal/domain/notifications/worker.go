package notifications

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WorkerOptions controls durable claiming, SMTP, and retry behavior.
type WorkerOptions struct {
	FromName      string
	FromAddress   string
	ReplyTo       string
	Host          string
	Port          int
	Username      string
	Password      string
	TLSMode       string
	Timeout       time.Duration
	PollInterval  time.Duration
	BatchSize     int
	LeaseDuration time.Duration
	MaxAttempts   int
	RetryBase     time.Duration
}

type smtpEnvelope struct {
	MessageID string
	FromName  string
	From      string
	ReplyTo   string
	ToName    string
	To        string
	Subject   string
	TextBody  string
	HTMLBody  string
}

type smtpSender interface {
	Send(context.Context, smtpEnvelope) error
}

// Worker dispatches pending deliveries from PostgreSQL.
type Worker struct {
	db     *gorm.DB
	opts   WorkerOptions
	logger *slog.Logger
	sender smtpSender
	now    func() time.Time
}

// NewWorker constructs the notification dispatcher.
func NewWorker(db *gorm.DB, opts WorkerOptions, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{db: db, opts: opts, logger: logger, sender: &networkSMTPSender{opts: opts}, now: time.Now}
}

// Run polls until the context is canceled.
func (w *Worker) Run(ctx context.Context) error {
	if w.opts.BatchSize <= 0 {
		w.opts.BatchSize = 25
	}
	if w.opts.PollInterval <= 0 {
		w.opts.PollInterval = 5 * time.Second
	}
	if w.opts.LeaseDuration <= 0 {
		w.opts.LeaseDuration = time.Minute
	}
	if w.opts.MaxAttempts <= 0 {
		w.opts.MaxAttempts = 8
	}
	if w.opts.RetryBase <= 0 {
		w.opts.RetryBase = time.Minute
	}
	w.logger.Info("notification worker started", "batch_size", w.opts.BatchSize)
	ticker := time.NewTicker(w.opts.PollInterval)
	defer ticker.Stop()
	for {
		if err := w.runOnce(ctx); err != nil && ctx.Err() == nil {
			w.logger.Error("notification dispatch failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) error {
	deliveries, err := w.claim(ctx, w.now().UTC())
	if err != nil {
		return err
	}
	for _, delivery := range deliveries {
		if ctx.Err() != nil {
			return nil
		}
		owned, err := w.renewClaim(ctx, delivery)
		if err != nil {
			return fmt.Errorf("renew notification claim %s: %w", delivery.ID, err)
		}
		if !owned {
			continue
		}
		var message Message
		if err := w.db.WithContext(ctx).First(&message, "id = ?", delivery.MessageID).Error; err != nil {
			w.completeFailure(ctx, delivery, fmt.Errorf("load message: %w", err))
			continue
		}
		envelope := smtpEnvelope{MessageID: delivery.ID + "@promptgate", FromName: w.opts.FromName, From: w.opts.FromAddress, ReplyTo: w.opts.ReplyTo, ToName: delivery.RecipientName, To: delivery.RecipientEmail, Subject: message.Subject, TextBody: message.TextBody, HTMLBody: message.HTMLBody}
		sendCtx := ctx
		cancelSend := func() {}
		if w.opts.Timeout > 0 {
			sendCtx, cancelSend = context.WithTimeout(ctx, w.opts.Timeout)
		}
		err = w.sender.Send(sendCtx, envelope)
		cancelSend()
		if err != nil {
			w.completeFailure(ctx, delivery, err)
			continue
		}
		now := w.now().UTC()
		if err := w.db.WithContext(ctx).Model(&Delivery{}).Where("id = ? AND status = ? AND attempts = ?", delivery.ID, DeliveryProcessing, delivery.Attempts).Updates(map[string]any{"status": DeliverySent, "sent_at": now, "last_attempt_at": now, "lease_until": nil, "last_error": ""}).Error; err != nil {
			w.logger.Error("failed to mark notification sent", "delivery_id", delivery.ID, "error", err)
		}
	}
	return nil
}

// renewClaim fences a claimed delivery immediately before SMTP. A delivery
// whose lease expired and was reclaimed has a newer attempt number, so an old
// worker must skip it instead of sending a duplicate.
func (w *Worker) renewClaim(ctx context.Context, delivery Delivery) (bool, error) {
	now := w.now().UTC()
	result := w.db.WithContext(ctx).Model(&Delivery{}).
		Where("id = ? AND status = ? AND attempts = ?", delivery.ID, DeliveryProcessing, delivery.Attempts).
		Update("lease_until", now.Add(w.opts.LeaseDuration))
	return result.RowsAffected == 1, result.Error
}

func (w *Worker) claim(ctx context.Context, now time.Time) ([]Delivery, error) {
	claimed := make([]Delivery, 0, w.opts.BatchSize)
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("(status = ? OR (status = ? AND lease_until < ?)) AND next_attempt_at <= ?", DeliveryPending, DeliveryProcessing, now, now).Order("next_attempt_at ASC").Limit(w.opts.BatchSize)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if err := query.Find(&claimed).Error; err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil
		}
		ids := make([]string, len(claimed))
		for i := range claimed {
			ids[i] = claimed[i].ID
			claimed[i].Status = DeliveryProcessing
			lease := now.Add(w.opts.LeaseDuration)
			claimed[i].LeaseUntil = &lease
			claimed[i].Attempts++
		}
		return tx.Model(&Delivery{}).Where("id IN ?", ids).Updates(map[string]any{"status": DeliveryProcessing, "lease_until": now.Add(w.opts.LeaseDuration), "attempts": gorm.Expr("attempts + 1"), "last_attempt_at": now}).Error
	})
	return claimed, err
}

func (w *Worker) completeFailure(ctx context.Context, delivery Delivery, sendErr error) {
	now := w.now().UTC()
	status := DeliveryPending
	next := now.Add(w.retryDelay(delivery.Attempts))
	if delivery.Attempts >= w.opts.MaxAttempts {
		status = DeliveryFailed
	}
	errorText := sendErr.Error()
	if len(errorText) > 2000 {
		errorText = errorText[:2000]
	}
	if err := w.db.WithContext(ctx).Model(&Delivery{}).Where("id = ? AND status = ? AND attempts = ?", delivery.ID, DeliveryProcessing, delivery.Attempts).Updates(map[string]any{"status": status, "next_attempt_at": next, "lease_until": nil, "last_attempt_at": now, "last_error": errorText}).Error; err != nil {
		w.logger.Error("failed to record notification failure", "delivery_id", delivery.ID, "error", err)
	}
}

func (w *Worker) retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 8 {
		shift = 8
	}
	return w.opts.RetryBase * time.Duration(1<<shift)
}

type networkSMTPSender struct{ opts WorkerOptions }

func (s *networkSMTPSender) Send(ctx context.Context, envelope smtpEnvelope) error {
	address := net.JoinHostPort(s.opts.Host, strconv.Itoa(s.opts.Port))
	dialer := &net.Dialer{Timeout: s.opts.Timeout}
	var connection net.Conn
	var err error
	if s.opts.TLSMode == "tls" {
		connection, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: s.opts.Host, MinVersion: tls.VersionTLS12})
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("connect smtp: %w", err)
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else if s.opts.Timeout > 0 {
		_ = connection.SetDeadline(time.Now().Add(s.opts.Timeout))
	}
	client, err := smtp.NewClient(connection, s.opts.Host)
	if err != nil {
		return fmt.Errorf("initialize smtp: %w", err)
	}
	defer client.Close()
	if s.opts.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("smtp server does not support STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: s.opts.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start smtp tls: %w", err)
		}
	}
	if s.opts.Username != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("smtp server does not support authentication")
		}
		if err := client.Auth(smtp.PlainAuth("", s.opts.Username, s.opts.Password, s.opts.Host)); err != nil {
			return fmt.Errorf("authenticate smtp: %w", err)
		}
	}
	if err := client.Mail(envelope.From); err != nil {
		return fmt.Errorf("set smtp sender: %w", err)
	}
	if err := client.Rcpt(envelope.To); err != nil {
		return fmt.Errorf("set smtp recipient: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("start smtp data: %w", err)
	}
	payload := buildMIMEMessage(envelope)
	if _, err := io.Copy(wc, bytes.NewReader(payload)); err != nil {
		_ = wc.Close()
		return fmt.Errorf("write smtp data: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("finish smtp data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit smtp: %w", err)
	}
	return nil
}

func buildMIMEMessage(envelope smtpEnvelope) []byte {
	boundary := "promptgate-" + strings.ReplaceAll(envelope.MessageID, "@", "-")
	fromAddress := mail.Address{Name: sanitizeHeaderName(envelope.FromName), Address: envelope.From}
	toAddress := mail.Address{Name: sanitizeHeaderName(envelope.ToName), Address: envelope.To}
	from := fromAddress.String()
	to := toAddress.String()
	var b strings.Builder
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", envelope.Subject),
		"Message-ID: <" + envelope.MessageID + ">",
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=\"" + boundary + "\"",
	}
	if envelope.ReplyTo != "" {
		headers = append(headers, "Reply-To: "+envelope.ReplyTo)
	}
	for _, header := range headers {
		b.WriteString(header)
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n--" + boundary + "\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(wrapBase64(envelope.TextBody))
	b.WriteString("\r\n--" + boundary + "\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(wrapBase64(envelope.HTMLBody))
	b.WriteString("\r\n--" + boundary + "--\r\n")
	return []byte(b.String())
}

func sanitizeHeaderName(value string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(value))
}

func wrapBase64(value string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(value))
	var b strings.Builder
	for len(encoded) > 76 {
		b.WriteString(encoded[:76])
		b.WriteString("\r\n")
		encoded = encoded[76:]
	}
	b.WriteString(encoded)
	return b.String()
}
