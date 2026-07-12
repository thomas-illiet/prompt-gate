package runtime

import (
	"context"
	"encoding/json"
	"strings"

	cdrslog "cdr.dev/slog/v3"
	"github.com/coder/aibridge/recorder"
)

const (
	embeddingTokenSourceProviderUsage   = "provider_usage"
	embeddingTokenSourceProviderMissing = "provider_usage_missing"
	embeddingTokenWarningTotalMismatch  = "embedding_total_tokens_mismatch"
)

// recordTokenUsage extracts embedding token usage from a successful upstream response.
func (i *embeddingInterceptor) recordTokenUsage(ctx context.Context, body []byte) {
	if i.recorder == nil {
		return
	}

	responseID := i.id.String()
	metadata := recorder.Metadata{
		"type": "embedding",
	}

	var payload embeddingResponsePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		i.logger.Warn(ctx, "failed to decode embeddings usage", cdrslog.Error(err), cdrslog.F("interception_id", i.id.String()))
	} else {
		if id := strings.TrimSpace(payload.ID); id != "" {
			responseID = id
		}
		inputTokens, warning := embeddingProviderInputTokens(payload.Usage)
		if inputTokens > 0 {
			metadata["token_source"] = embeddingTokenSourceProviderUsage
			if warning != "" {
				metadata["token_warning"] = warning
				i.logger.Warn(ctx, "embeddings provider usage mismatch", cdrslog.F("interception_id", i.id.String()), cdrslog.F("model", i.Model()), cdrslog.F("token_warning", warning))
			}
			i.recordEmbeddingTokens(ctx, responseID, inputTokens, metadata)
			return
		}
	}

	metadata["token_source"] = embeddingTokenSourceProviderMissing
	i.recordEmbeddingTokens(ctx, responseID, 0, metadata)
}

// recordEmbeddingTokens persists embedding token totals through the aibridge recorder.
func (i *embeddingInterceptor) recordEmbeddingTokens(ctx context.Context, responseID string, inputTokens int64, metadata recorder.Metadata) {
	_ = i.recorder.RecordTokenUsage(ctx, &recorder.TokenUsageRecord{
		InterceptionID: i.id.String(),
		MsgID:          responseID,
		Input:          inputTokens,
		Output:         0,
		Metadata:       metadata,
	})
}

type embeddingResponsePayload struct {
	ID    string                 `json:"id"`
	Usage embeddingResponseUsage `json:"usage"`
}

type embeddingResponseUsage struct {
	PromptTokens *int64 `json:"prompt_tokens"`
	InputTokens  *int64 `json:"input_tokens"`
	TotalTokens  *int64 `json:"total_tokens"`
}

// embeddingProviderInputTokens normalizes provider usage fields into embedding input tokens.
func embeddingProviderInputTokens(usage embeddingResponseUsage) (int64, string) {
	inputTokens := int64(0)
	switch {
	case usage.PromptTokens != nil && *usage.PromptTokens > 0:
		inputTokens = *usage.PromptTokens
	case usage.InputTokens != nil && *usage.InputTokens > 0:
		inputTokens = *usage.InputTokens
	case usage.PromptTokens == nil && usage.InputTokens == nil && usage.TotalTokens != nil && *usage.TotalTokens > 0:
		inputTokens = *usage.TotalTokens
	}

	if inputTokens > 0 && usage.TotalTokens != nil && *usage.TotalTokens > 0 && *usage.TotalTokens != inputTokens {
		return inputTokens, embeddingTokenWarningTotalMismatch
	}
	return inputTokens, ""
}
