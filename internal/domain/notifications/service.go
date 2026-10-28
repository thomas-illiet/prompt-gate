package notifications

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrMessageNotFound = errors.New("notification message not found")
	ErrInvalidAddress  = errors.New("invalid email address")
	ErrInvalidSubject  = errors.New("invalid email subject")
	ErrInvalidBody     = errors.New("invalid email body")
	ErrInvalidAudience = errors.New("invalid email audience")
	ErrInvalidSort     = errors.New("invalid_sort")
)

const maxSubjectLength = 255

type recipientRow struct {
	ID        string
	Email     string
	Name      string
	IsActive  bool
	Role      string
	ExpiresAt *time.Time
}

// Recipient is one validated, resolved mail recipient.
type Recipient struct {
	UserID *string `json:"userId"`
	Email  string  `json:"email"`
	Name   string  `json:"name"`
}

// AudienceRequest selects recipients for an admin-composed message.
type AudienceRequest struct {
	Mode            string   `json:"mode"`
	ExternalAddress string   `json:"externalAddress"`
	UserIDs         []string `json:"userIds"`
	GroupIDs        []string `json:"groupIds"`
}

// AudienceResult reports resolution without exposing ignored invalid addresses.
type AudienceResult struct {
	Recipients []Recipient `json:"recipients"`
	Valid      int         `json:"valid"`
	Ignored    int         `json:"ignored"`
}

// ManualMessageRequest is the admin compose contract.
type ManualMessageRequest struct {
	Subject  string          `json:"subject"`
	Markdown string          `json:"markdown"`
	Audience AudienceRequest `json:"audience"`
}

// ListParams controls administration history queries.
type ListParams struct {
	Page     int
	PageSize int
	Search   string
	Kind     Kind
	Status   DeliveryStatus
	SortDir  string
}

// Service owns durable notification composition and querying.
type Service struct {
	db            *gorm.DB
	renderer      *Renderer
	enabled       bool
	frontendURL   string
	inputRate     float64
	outputRate    float64
	embeddingRate float64
}

// NewService constructs a notification service.
func NewService(db *gorm.DB, enabled bool, frontendURL string) *Service {
	return &Service{db: db, renderer: NewRenderer(), enabled: enabled, frontendURL: strings.TrimRight(frontendURL, "/")}
}

// Enabled reports whether automatic and manual email enqueueing is active.
func (s *Service) Enabled() bool { return s != nil && s.enabled }

// SetUsageCostRates configures estimated USD-per-million-token report rates.
func (s *Service) SetUsageCostRates(input, output, embedding float64) {
	s.inputRate, s.outputRate, s.embeddingRate = input, output, embedding
}

// AutoMigrate creates notification persistence tables.
func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&Message{}, &Delivery{}, &QuotaState{})
}

// Preview safely renders a complete manual email without creating a message.
func (s *Service) Preview(subject, markdown string) (textBody, htmlBody string, err error) {
	subject, err = normalizeSubject(subject)
	if err != nil {
		return "", "", err
	}
	return s.renderer.RenderTemplate(KindManual, "", subject, markdown, s.frontendURL)
}

func normalizeSubject(subject string) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" || len(subject) > maxSubjectLength || strings.ContainsAny(subject, "\r\n") {
		return "", ErrInvalidSubject
	}
	return subject, nil
}

func normalizeAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", ErrInvalidAddress
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Name != "" || !strings.EqualFold(parsed.Address, value) {
		return "", ErrInvalidAddress
	}
	return strings.ToLower(parsed.Address), nil
}

// ResolveAudience resolves users/groups/all/external into valid recipients.
func (s *Service) ResolveAudience(ctx context.Context, input AudienceRequest) (AudienceResult, error) {
	mode := strings.TrimSpace(input.Mode)
	if mode == "external" {
		address, err := normalizeAddress(input.ExternalAddress)
		if err != nil {
			return AudienceResult{}, err
		}
		return AudienceResult{Recipients: []Recipient{{Email: address}}, Valid: 1}, nil
	}

	query := s.db.WithContext(ctx).Table("users").
		Select("users.id, users.email, users.name, users.is_active, users.role, users.expires_at").
		Where("users.type = ?", "user")
	switch mode {
	case "users":
		if len(input.UserIDs) == 0 {
			return AudienceResult{}, ErrInvalidAudience
		}
		query = query.Where("users.id IN ?", input.UserIDs)
	case "groups":
		if len(input.GroupIDs) == 0 {
			return AudienceResult{}, ErrInvalidAudience
		}
		query = query.Joins("JOIN access_group_members agm ON agm.user_id = users.id").Where("agm.group_id IN ?", input.GroupIDs).Distinct()
	case "all":
	default:
		return AudienceResult{}, ErrInvalidAudience
	}

	var rows []recipientRow
	if err := query.Order("users.preferred_username ASC").Find(&rows).Error; err != nil {
		return AudienceResult{}, fmt.Errorf("resolve notification audience: %w", err)
	}
	result := AudienceResult{Recipients: make([]Recipient, 0, len(rows))}
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		address, err := normalizeAddress(row.Email)
		if err != nil {
			result.Ignored++
			continue
		}
		if _, exists := seen[row.ID]; exists {
			continue
		}
		seen[row.ID] = struct{}{}
		id := row.ID
		result.Recipients = append(result.Recipients, Recipient{UserID: &id, Email: address, Name: strings.TrimSpace(row.Name)})
	}
	result.Valid = len(result.Recipients)
	return result, nil
}

// EnqueueManual resolves the audience and creates one message plus deliveries.
func (s *Service) EnqueueManual(ctx context.Context, request ManualMessageRequest, actorUserID *string, idempotencyKey string) (MessageDetail, AudienceResult, error) {
	if !s.Enabled() {
		return MessageDetail{}, AudienceResult{}, errors.New("notifications disabled")
	}
	subject, err := normalizeSubject(request.Subject)
	if err != nil {
		return MessageDetail{}, AudienceResult{}, err
	}
	textBody, htmlBody, err := s.renderer.RenderTemplate(KindManual, "", subject, request.Markdown, s.frontendURL)
	if err != nil {
		return MessageDetail{}, AudienceResult{}, err
	}
	audience, err := s.ResolveAudience(ctx, request.Audience)
	if err != nil {
		return MessageDetail{}, AudienceResult{}, err
	}
	if audience.Valid == 0 {
		return MessageDetail{}, audience, ErrInvalidAudience
	}
	var messageID string
	hash := sha256.Sum256([]byte(strings.TrimSpace(idempotencyKey)))
	hashedKey := hex.EncodeToString(hash[:])
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing Message
		if err := tx.Where("idempotency_key = ?", hashedKey).Take(&existing).Error; err == nil {
			messageID = existing.ID
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		message := Message{Kind: KindManual, Subject: subject, MarkdownBody: strings.TrimSpace(request.Markdown), TextBody: textBody, HTMLBody: htmlBody, CreatedByUserID: actorUserID, IdempotencyKey: &hashedKey, Audience: strings.TrimSpace(request.Audience.Mode)}
		created := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).Create(&message)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var existing Message
			if err := tx.Where("idempotency_key = ?", hashedKey).Take(&existing).Error; err != nil {
				return err
			}
			messageID = existing.ID
			return nil
		}
		messageID = message.ID
		for _, recipient := range audience.Recipients {
			var dedupe *string
			if strings.TrimSpace(idempotencyKey) != "" {
				key := "manual:" + strings.TrimSpace(idempotencyKey) + ":" + recipient.Email
				if recipient.UserID != nil {
					key = "manual:" + strings.TrimSpace(idempotencyKey) + ":" + *recipient.UserID
				}
				dedupe = &key
			}
			delivery := Delivery{MessageID: message.ID, UserID: recipient.UserID, RecipientEmail: recipient.Email, RecipientName: recipient.Name, DeduplicationKey: dedupe}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&delivery).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return MessageDetail{}, audience, fmt.Errorf("enqueue manual notification: %w", err)
	}
	detail, err := s.GetMessage(ctx, messageID)
	return detail, audience, err
}

// EnqueueAutomaticTx creates one idempotent automatic delivery inside tx.
func (s *Service) EnqueueAutomaticTx(ctx context.Context, tx *gorm.DB, kind Kind, userID, dedupeKey, subject, markdown string) error {
	if !s.Enabled() {
		return nil
	}
	subject, err := normalizeSubject(subject)
	if err != nil {
		return err
	}
	var recipient recipientRow
	if err := tx.WithContext(ctx).Table("users").Select("id, email, name").Where("id = ? AND type = ?", userID, "user").Take(&recipient).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	address, err := normalizeAddress(recipient.Email)
	if err != nil {
		return nil
	}
	textBody, htmlBody, err := s.renderer.RenderTemplate(kind, recipient.Name, subject, markdown, s.frontendURL)
	if err != nil {
		return err
	}
	message := Message{Kind: kind, Subject: subject, MarkdownBody: strings.TrimSpace(markdown), TextBody: textBody, HTMLBody: htmlBody, Audience: "automatic"}
	if err := tx.WithContext(ctx).Create(&message).Error; err != nil {
		return err
	}
	key := dedupeKey
	delivery := Delivery{MessageID: message.ID, UserID: &recipient.ID, RecipientEmail: address, RecipientName: recipient.Name, DeduplicationKey: &key}
	result := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&delivery)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return tx.WithContext(ctx).Delete(&message).Error
	}
	return nil
}

// ListMessages returns aggregate delivery counters for admin history.
func (s *Service) ListMessages(ctx context.Context, params ListParams) (ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		return ListResult{}, ErrInvalidSort
	}
	base := s.db.WithContext(ctx).Model(&Message{})
	if search := strings.TrimSpace(params.Search); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		base = base.Where("LOWER(subject) LIKE ?", like)
	}
	if params.Kind != "" {
		base = base.Where("kind = ?", params.Kind)
	}
	if params.Status != "" {
		base = base.Where("EXISTS (SELECT 1 FROM deliveries d WHERE d.message_id = messages.id AND d.status = ?)", params.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return ListResult{}, err
	}
	var messages []Message
	if err := base.Order("created_at " + params.SortDir).Order("id " + params.SortDir).Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).Find(&messages).Error; err != nil {
		return ListResult{}, err
	}
	items := make([]MessageSummary, 0, len(messages))
	for _, message := range messages {
		var deliveries []Delivery
		if err := s.db.WithContext(ctx).Where("message_id = ?", message.ID).Find(&deliveries).Error; err != nil {
			return ListResult{}, err
		}
		items = append(items, summarizeMessage(message, deliveries))
	}
	return ListResult{Items: items, Page: params.Page, PageSize: params.PageSize, Total: total}, nil
}

// GetMessage returns content and sorted deliveries for one message.
func (s *Service) GetMessage(ctx context.Context, id string) (MessageDetail, error) {
	var message Message
	if err := s.db.WithContext(ctx).Preload("Deliveries", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).First(&message, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MessageDetail{}, ErrMessageNotFound
		}
		return MessageDetail{}, err
	}
	return messageDetail(message), nil
}

// RetryFailed resets failed deliveries of one message.
func (s *Service) RetryFailed(ctx context.Context, id string, now time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&Message{}).Where("id = ?", id).Count(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			return ErrMessageNotFound
		}
		result := tx.Model(&Delivery{}).Where("message_id = ? AND status = ?", id, DeliveryFailed).Updates(map[string]any{"status": DeliveryPending, "next_attempt_at": now.UTC(), "lease_until": nil, "last_error": ""})
		count = result.RowsAffected
		return result.Error
	})
	return count, err
}

// Cleanup removes complete notification history older than retention.
func (s *Service) Cleanup(ctx context.Context, retention time.Duration, now time.Time) (int64, error) {
	if retention <= 0 {
		return 0, nil
	}
	const cleanupBatchSize = 500
	cutoff := now.UTC().Add(-retention)
	var removed int64
	for {
		batchSize := 0
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var messageIDs []string
			if err := tx.Model(&Message{}).Where("created_at < ?", cutoff).Order("created_at ASC").Limit(cleanupBatchSize).Pluck("id", &messageIDs).Error; err != nil {
				return err
			}
			batchSize = len(messageIDs)
			if batchSize == 0 {
				return nil
			}
			// Delete explicitly instead of relying on database-specific cascade setup.
			if err := tx.Where("message_id IN ?", messageIDs).Delete(&Delivery{}).Error; err != nil {
				return err
			}
			result := tx.Where("id IN ?", messageIDs).Delete(&Message{})
			removed += result.RowsAffected
			return result.Error
		})
		if err != nil {
			return removed, err
		}
		if batchSize < cleanupBatchSize {
			return removed, nil
		}
	}
}

func summarizeMessage(message Message, deliveries []Delivery) MessageSummary {
	result := MessageSummary{ID: message.ID, Kind: message.Kind, Subject: message.Subject, Audience: message.Audience, Recipients: int64(len(deliveries)), CreatedAt: message.CreatedAt}
	for _, delivery := range deliveries {
		switch delivery.Status {
		case DeliveryPending:
			result.Pending++
		case DeliveryProcessing:
			result.Processing++
		case DeliverySent:
			result.Sent++
		case DeliveryFailed:
			result.Failed++
		}
	}
	return result
}

func messageDetail(message Message) MessageDetail {
	sort.Slice(message.Deliveries, func(i, j int) bool { return message.Deliveries[i].CreatedAt.Before(message.Deliveries[j].CreatedAt) })
	deliveries := make([]DeliveryResponse, 0, len(message.Deliveries))
	for _, d := range message.Deliveries {
		deliveries = append(deliveries, DeliveryResponse{ID: d.ID, UserID: d.UserID, RecipientEmail: d.RecipientEmail, RecipientName: d.RecipientName, Status: d.Status, Attempts: d.Attempts, NextAttemptAt: d.NextAttemptAt, SentAt: d.SentAt, LastAttemptAt: d.LastAttemptAt, LastError: d.LastError})
	}
	return MessageDetail{MessageSummary: summarizeMessage(message, message.Deliveries), MarkdownBody: message.MarkdownBody, TextBody: message.TextBody, HTMLBody: message.HTMLBody, Deliveries: deliveries}
}
