package proxy

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func processUsageEventTx(tx *gorm.DB, event UsageEvent, redisMessageID string) error {
	if strings.TrimSpace(event.EventID) == "" {
		event.EventID = redisMessageID
	}
	processed := ProcessedUsageEvent{
		EventID: event.EventID, RedisMessageID: redisMessageID,
		Type: string(event.Type), CreatedAt: event.CreatedAt, ProcessedAt: time.Now().UTC(),
	}
	if processed.CreatedAt.IsZero() {
		processed.CreatedAt = processed.ProcessedAt
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&processed)
	if result.Error != nil {
		return fmt.Errorf("mark usage event processed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errUsageEventAlreadyProcessed
	}
	return processUsageEvent(tx, event)
}

func processUsageEvent(tx *gorm.DB, event UsageEvent) error {
	switch event.Type {
	case UsageEventInterceptionStarted:
		return processInterceptionStarted(tx, event.InterceptionStarted)
	case UsageEventInterceptionEnded:
		return processInterceptionEnded(tx, event.InterceptionEnded)
	case UsageEventTokenUsage:
		return processTokenUsage(tx, event.TokenUsage)
	case UsageEventPromptUsage:
		return processPromptUsage(tx, event.PromptUsage)
	case UsageEventToolUsage:
		return processToolUsage(tx, event.ToolUsage)
	default:
		return fmt.Errorf("unknown usage event type %q", event.Type)
	}
}

func processInterceptionStarted(tx *gorm.DB, payload *InterceptionStartedEvent) error {
	if payload == nil {
		return errors.New("missing interception_started payload")
	}
	record := Interception{
		ID: payload.ID, InitiatorID: payload.InitiatorID, Provider: payload.Provider,
		ProviderType: payload.ProviderType, Model: payload.Model, ClientIP: payload.ClientIP,
		StartedAt: timestamp(payload.StartedAt), Metadata: payload.Metadata,
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
	if result.Error != nil {
		return fmt.Errorf("record interception: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}
	return aggregateInterceptionStarted(tx, record)
}

func processInterceptionEnded(tx *gorm.DB, payload *InterceptionEndedEvent) error {
	if payload == nil {
		return errors.New("missing interception_ended payload")
	}
	interception, err := loadUsageInterception(tx, payload.ID)
	if err != nil {
		return err
	}
	if interception.EndedAt != nil {
		return nil
	}
	endedAt := timestamp(payload.EndedAt)
	if err := tx.Model(&Interception{}).Where("id = ?", payload.ID).Update("ended_at", endedAt).Error; err != nil {
		return fmt.Errorf("record interception end: %w", err)
	}
	interception.EndedAt = &endedAt
	return aggregateInterceptionDuration(tx, interception)
}

func processTokenUsage(tx *gorm.DB, payload *TokenUsageEvent) error {
	if payload == nil {
		return errors.New("missing token_usage payload")
	}
	interception, err := loadUsageInterception(tx, payload.InterceptionID)
	if err != nil {
		return err
	}
	record := TokenUsage{
		ID: uuid.NewString(), InterceptionID: payload.InterceptionID,
		ProviderResponseID: payload.ProviderResponseID, InputTokens: payload.InputTokens,
		OutputTokens: payload.OutputTokens, CacheReadInputTokens: payload.CacheReadInputTokens,
		CacheWriteInputTokens: payload.CacheWriteInputTokens, Type: payload.Type,
		Metadata: payload.Metadata, CreatedAt: timestamp(payload.CreatedAt),
	}
	if record.Type == "" {
		record.Type = tokenUsageTypeCompletion
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("record token usage: %w", err)
	}
	return aggregateTokenUsage(tx, interception, record)
}

func processPromptUsage(tx *gorm.DB, payload *PromptUsageEvent) error {
	if payload == nil {
		return errors.New("missing prompt_usage payload")
	}
	interception, err := loadUsageInterception(tx, payload.InterceptionID)
	if err != nil {
		return err
	}
	record := UserPrompt{
		ID: uuid.NewString(), InterceptionID: payload.InterceptionID,
		ProviderResponseID: payload.ProviderResponseID, Prompt: payload.Prompt,
		Metadata: payload.Metadata, CreatedAt: timestamp(payload.CreatedAt),
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("record prompt usage: %w", err)
	}
	return aggregatePromptUsage(tx, interception, record)
}

func processToolUsage(tx *gorm.DB, payload *ToolUsageEvent) error {
	if payload == nil {
		return errors.New("missing tool_usage payload")
	}
	interception, err := loadUsageInterception(tx, payload.InterceptionID)
	if err != nil {
		return err
	}
	record := ToolUsage{
		ID: uuid.NewString(), InterceptionID: payload.InterceptionID,
		ProviderResponseID: payload.ProviderResponseID, ServerURL: payload.ServerURL,
		Tool: payload.Tool, Input: payload.Input, Injected: payload.Injected,
		InvocationError: payload.InvocationError, Metadata: payload.Metadata,
		CreatedAt: timestamp(payload.CreatedAt),
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("record tool usage: %w", err)
	}
	return aggregateToolUsage(tx, interception, record)
}
