package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	aibrecorder "github.com/coder/aibridge/recorder"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Recorder struct {
	db *gorm.DB
}

type tokenUsageEndpointMigration struct {
	TokenUsage
	Endpoint string `gorm:"column:endpoint"`
}

func (tokenUsageEndpointMigration) TableName() string {
	return "token_usages"
}

// NewRecorder creates a GORM-backed proxy recorder.
func NewRecorder(db *gorm.DB) *Recorder {
	return &Recorder{db: db}
}

// AutoMigrate migrates proxy recorder tables.
func AutoMigrate(ctx context.Context, db *gorm.DB) error {
	migrator := db.WithContext(ctx).Migrator()
	if migrator.HasTable("model_thoughts") {
		if err := migrator.DropTable("model_thoughts"); err != nil {
			return fmt.Errorf("drop model thoughts table: %w", err)
		}
	}

	if err := db.WithContext(ctx).AutoMigrate(
		&Interception{},
		&TokenUsage{},
		&UserPrompt{},
		&ToolUsage{},
	); err != nil {
		return err
	}

	if err := db.WithContext(ctx).
		Model(&TokenUsage{}).
		Where("metadata LIKE ? OR metadata LIKE ?", `%"type":"embedding"%`, `%"endpoint":"/embeddings"%`).
		Update("type", tokenUsageTypeEmbedding).Error; err != nil {
		return fmt.Errorf("backfill token usage type from metadata: %w", err)
	}

	if migrator.HasColumn(&tokenUsageEndpointMigration{}, "endpoint") {
		if err := db.WithContext(ctx).
			Table("token_usages").
			Where("endpoint = ?", tokenUsageEndpointEmbeddings).
			Update("type", tokenUsageTypeEmbedding).Error; err != nil {
			return fmt.Errorf("backfill token usage type from endpoint: %w", err)
		}
		if err := db.WithContext(ctx).Exec("ALTER TABLE token_usages DROP COLUMN endpoint").Error; err != nil {
			return fmt.Errorf("drop token usage endpoint column: %w", err)
		}
	}
	return nil
}

// RecordInterception persists the start of a proxied interaction.
func (r *Recorder) RecordInterception(ctx context.Context, req *aibrecorder.InterceptionRecord) error {
	startedAt := req.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		return err
	}
	record := Interception{
		ID:           req.ID,
		InitiatorID:  req.InitiatorID,
		Provider:     req.ProviderName,
		ProviderType: req.Provider,
		Model:        req.Model,
		StartedAt:    startedAt,
		Metadata:     metadata,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("record interception: %w", err)
	}
	return nil
}

// RecordInterceptionEnded updates an interception with completion data.
func (r *Recorder) RecordInterceptionEnded(ctx context.Context, req *aibrecorder.InterceptionRecordEnded) error {
	endedAt := req.EndedAt
	if endedAt.IsZero() {
		endedAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).
		Model(&Interception{}).
		Where("id = ?", req.ID).
		Update("ended_at", endedAt).Error; err != nil {
		return fmt.Errorf("record interception end: %w", err)
	}
	return nil
}

// RecordTokenUsage persists token usage for a proxied request.
func (r *Recorder) RecordTokenUsage(ctx context.Context, req *aibrecorder.TokenUsageRecord) error {
	tokenType := metadataTokenUsageType(req.Metadata)
	metadata := mergeMetadata(req.Metadata, req.ExtraTokenTypes)
	record := TokenUsage{
		ID:                    uuid.NewString(),
		InterceptionID:        req.InterceptionID,
		ProviderResponseID:    req.MsgID,
		InputTokens:           req.Input,
		OutputTokens:          req.Output,
		CacheReadInputTokens:  req.CacheReadInputTokens,
		CacheWriteInputTokens: req.CacheWriteInputTokens,
		Type:                  tokenType,
		Metadata:              metadata,
		CreatedAt:             timestamp(req.CreatedAt),
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("record token usage: %w", err)
	}
	return nil
}

// RecordPromptUsage persists a user prompt observed by the proxy.
func (r *Recorder) RecordPromptUsage(ctx context.Context, req *aibrecorder.PromptUsageRecord) error {
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		return err
	}
	record := UserPrompt{
		ID:                 uuid.NewString(),
		InterceptionID:     req.InterceptionID,
		ProviderResponseID: req.MsgID,
		Prompt:             req.Prompt,
		Metadata:           metadata,
		CreatedAt:          timestamp(req.CreatedAt),
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("record prompt usage: %w", err)
	}
	return nil
}

// RecordToolUsage persists tool invocation data observed by the proxy.
func (r *Recorder) RecordToolUsage(ctx context.Context, req *aibrecorder.ToolUsageRecord) error {
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		return err
	}
	input, err := json.Marshal(req.Args)
	if err != nil {
		return fmt.Errorf("marshal tool input: %w", err)
	}
	var invocationError *string
	if req.InvocationError != nil {
		value := req.InvocationError.Error()
		invocationError = &value
	}
	record := ToolUsage{
		ID:                 uuid.NewString(),
		InterceptionID:     req.InterceptionID,
		ProviderResponseID: req.MsgID,
		ServerURL:          req.ServerURL,
		Tool:               req.Tool,
		Input:              string(input),
		Injected:           req.Injected,
		InvocationError:    invocationError,
		Metadata:           metadata,
		CreatedAt:          timestamp(req.CreatedAt),
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("record tool usage: %w", err)
	}
	return nil
}

// RecordModelThought satisfies the promptgate recorder interface without storing model thoughts.
func (r *Recorder) RecordModelThought(_ context.Context, _ *aibrecorder.ModelThoughtRecord) error {
	return nil
}

// marshalMetadata serializes recorder metadata to JSON.
func marshalMetadata(metadata aibrecorder.Metadata) (string, error) {
	if metadata == nil {
		return "{}", nil
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(b), nil
}

// mergeMetadata combines recorder metadata with numeric extras.
func mergeMetadata(metadata aibrecorder.Metadata, extra map[string]int64) string {
	merged := make(map[string]any, len(metadata)+len(extra))
	for k, v := range metadata {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	b, _ := json.Marshal(merged)
	return string(b)
}

// metadataTokenUsageType extracts the token usage type from recorder metadata.
func metadataTokenUsageType(metadata aibrecorder.Metadata) string {
	if metadata == nil {
		return tokenUsageTypeCompletion
	}
	if tokenType, _ := metadata["type"].(string); strings.TrimSpace(tokenType) == tokenUsageTypeEmbedding {
		return tokenUsageTypeEmbedding
	}
	endpoint, _ := metadata["endpoint"].(string)
	if strings.TrimSpace(endpoint) == tokenUsageEndpointEmbeddings {
		return tokenUsageTypeEmbedding
	}
	return tokenUsageTypeCompletion
}

// timestamp normalizes zero timestamps to the current time.
func timestamp(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}
