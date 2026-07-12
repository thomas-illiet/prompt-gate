package proxy

import "time"

const (
	UsageEventsStream        = "promptgate:usage:events"
	UsageEventsConsumerGroup = "promptgate-workers"
	UsageEventPayloadField   = "payload"
)

type UsageEventType string

const (
	UsageEventInterceptionStarted UsageEventType = "interception_started"
	UsageEventInterceptionEnded   UsageEventType = "interception_ended"
	UsageEventTokenUsage          UsageEventType = "token_usage"
	UsageEventPromptUsage         UsageEventType = "prompt_usage"
	UsageEventToolUsage           UsageEventType = "tool_usage"
)

type UsageEvent struct {
	EventID             string                    `json:"eventId"`
	Type                UsageEventType            `json:"type"`
	CreatedAt           time.Time                 `json:"createdAt"`
	InterceptionStarted *InterceptionStartedEvent `json:"interceptionStarted,omitempty"`
	InterceptionEnded   *InterceptionEndedEvent   `json:"interceptionEnded,omitempty"`
	TokenUsage          *TokenUsageEvent          `json:"tokenUsage,omitempty"`
	PromptUsage         *PromptUsageEvent         `json:"promptUsage,omitempty"`
	ToolUsage           *ToolUsageEvent           `json:"toolUsage,omitempty"`
}

type InterceptionStartedEvent struct {
	ID                    string    `json:"id"`
	InitiatorID           string    `json:"initiatorId"`
	Provider              string    `json:"provider"`
	ProviderType          string    `json:"providerType"`
	Model                 string    `json:"model"`
	ClientIP              string    `json:"clientIp"`
	StartedAt             time.Time `json:"startedAt"`
	Metadata              string    `json:"metadata"`
	ClientSessionID       *string   `json:"clientSessionId,omitempty"`
	Client                string    `json:"client,omitempty"`
	UserAgent             string    `json:"userAgent,omitempty"`
	CorrelatingToolCallID *string   `json:"correlatingToolCallId,omitempty"`
	CredentialKind        string    `json:"credentialKind,omitempty"`
	CredentialHint        string    `json:"credentialHint,omitempty"`
}

type InterceptionEndedEvent struct {
	ID      string    `json:"id"`
	EndedAt time.Time `json:"endedAt"`
}

type TokenUsageEvent struct {
	InterceptionID        string    `json:"interceptionId"`
	ProviderResponseID    string    `json:"providerResponseId"`
	InputTokens           int64     `json:"inputTokens"`
	OutputTokens          int64     `json:"outputTokens"`
	CacheReadInputTokens  int64     `json:"cacheReadInputTokens"`
	CacheWriteInputTokens int64     `json:"cacheWriteInputTokens"`
	Type                  string    `json:"type"`
	Metadata              string    `json:"metadata"`
	CreatedAt             time.Time `json:"createdAt"`
}

type PromptUsageEvent struct {
	InterceptionID     string    `json:"interceptionId"`
	ProviderResponseID string    `json:"providerResponseId"`
	Prompt             string    `json:"prompt"`
	Metadata           string    `json:"metadata"`
	CreatedAt          time.Time `json:"createdAt"`
}

type ToolUsageEvent struct {
	InterceptionID     string    `json:"interceptionId"`
	ProviderResponseID string    `json:"providerResponseId"`
	ServerURL          *string   `json:"serverUrl,omitempty"`
	Tool               string    `json:"tool"`
	Input              string    `json:"input"`
	Injected           bool      `json:"injected"`
	InvocationError    *string   `json:"invocationError,omitempty"`
	Metadata           string    `json:"metadata"`
	CreatedAt          time.Time `json:"createdAt"`
}
