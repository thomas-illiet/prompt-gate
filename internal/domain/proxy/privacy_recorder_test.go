package proxy

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/requestprivacy"

	aibrecorder "github.com/coder/aibridge/recorder"
)

func TestPrivacyRecorderDropsContentAndSanitizesMetadataOnlyRecords(t *testing.T) {
	inner := &recordingPrivacyRecorder{}
	recorder := NewPrivacyRecorder(inner)
	ctx := clientip.ContextWithClientIP(requestprivacy.MarkMetadataOnly(context.Background()), "198.51.100.77")
	sessionID := "session-secret"
	toolCallID := "tool-secret"
	interception := &aibrecorder.InterceptionRecord{
		ID: "interception", InitiatorID: "account-id", Provider: "openai", ProviderName: "primary", Model: "model",
		Metadata: aibrecorder.Metadata{"email": "person@example.com"}, ClientSessionID: &sessionID,
		Client: "codex", UserAgent: "secret-agent", CorrelatingToolCallID: &toolCallID,
		CredentialKind: "centralized", CredentialHint: "ticket-secret",
	}
	tokens := &aibrecorder.TokenUsageRecord{
		InterceptionID: "interception", MsgID: "provider-secret", Input: 7, Output: 3,
		Metadata: aibrecorder.Metadata{"raw": "secret"}, ExtraTokenTypes: map[string]int64{"completion_reasoning": 2},
	}

	if err := recorder.RecordInterception(ctx, interception); err != nil {
		t.Fatalf("record interception: %v", err)
	}
	if err := recorder.RecordTokenUsage(ctx, tokens); err != nil {
		t.Fatalf("record tokens: %v", err)
	}
	_ = recorder.RecordPromptUsage(ctx, &aibrecorder.PromptUsageRecord{Prompt: "prompt-secret"})
	_ = recorder.RecordToolUsage(ctx, &aibrecorder.ToolUsageRecord{Args: map[string]any{"secret": true}})
	_ = recorder.RecordModelThought(ctx, &aibrecorder.ModelThoughtRecord{Content: "thought-secret"})

	got := inner.interception
	if got == nil {
		t.Fatal("interception was not delegated")
	}
	if got.ID != "interception" || got.InitiatorID != "account-id" || got.ProviderName != "primary" || got.Model != "model" {
		t.Fatalf("accounting metadata was not preserved: %#v", got)
	}
	if got.Metadata != nil || got.ClientSessionID != nil || got.Client != "" || got.UserAgent != "" || got.CorrelatingToolCallID != nil || got.CredentialHint != "" {
		t.Fatalf("identifying metadata was not removed: %#v", got)
	}
	if got.CredentialKind != "model_lab_ticket" {
		t.Fatalf("credential provenance was removed: %#v", got.CredentialKind)
	}
	if inner.clientIP != "" {
		t.Fatalf("metadata-only request persisted client IP %q", inner.clientIP)
	}
	if inner.tokens == nil || inner.tokens.Input != 7 || inner.tokens.Output != 3 || inner.tokens.MsgID != "" || inner.tokens.Metadata != nil {
		t.Fatalf("token record was not safely preserved: %#v", inner.tokens)
	}
	if !reflect.DeepEqual(inner.tokens.ExtraTokenTypes, map[string]int64{"completion_reasoning": 2}) {
		t.Fatalf("token subtype counts changed: %#v", inner.tokens.ExtraTokenTypes)
	}
	if inner.prompts != 0 || inner.tools != 0 || inner.thoughts != 0 {
		t.Fatalf("content records reached inner recorder: prompts=%d tools=%d thoughts=%d", inner.prompts, inner.tools, inner.thoughts)
	}
	if interception.Metadata["email"] != "person@example.com" || interception.ClientSessionID == nil || tokens.MsgID != "provider-secret" {
		t.Fatal("privacy recorder mutated caller-owned records")
	}
}

func TestPrivacyRecorderLeavesStandardRequestsUnchanged(t *testing.T) {
	inner := &recordingPrivacyRecorder{}
	recorder := NewPrivacyRecorder(inner)
	ctx := context.Background()
	interception := &aibrecorder.InterceptionRecord{ID: "id", Metadata: aibrecorder.Metadata{"email": "person@example.com"}}
	tokens := &aibrecorder.TokenUsageRecord{InterceptionID: "id", MsgID: "message", Metadata: aibrecorder.Metadata{"type": "completion"}}
	prompt := &aibrecorder.PromptUsageRecord{Prompt: "prompt"}
	tool := &aibrecorder.ToolUsageRecord{Tool: "lookup"}
	thought := &aibrecorder.ModelThoughtRecord{Content: "thinking"}

	_ = recorder.RecordInterception(ctx, interception)
	_ = recorder.RecordTokenUsage(ctx, tokens)
	_ = recorder.RecordPromptUsage(ctx, prompt)
	_ = recorder.RecordToolUsage(ctx, tool)
	_ = recorder.RecordModelThought(ctx, thought)

	if inner.interception != interception || inner.tokens != tokens || inner.lastPrompt != prompt || inner.lastTool != tool || inner.lastThought != thought {
		t.Fatal("standard request records were copied or changed")
	}
}

func TestPrivacyRecorderPropagatesAccountingErrors(t *testing.T) {
	want := errors.New("accounting unavailable")
	inner := &recordingPrivacyRecorder{interceptionError: want}
	recorder := NewPrivacyRecorder(inner)
	if err := recorder.RecordInterception(requestprivacy.MarkMetadataOnly(context.Background()), &aibrecorder.InterceptionRecord{}); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

type recordingPrivacyRecorder struct {
	interception      *aibrecorder.InterceptionRecord
	tokens            *aibrecorder.TokenUsageRecord
	lastPrompt        *aibrecorder.PromptUsageRecord
	lastTool          *aibrecorder.ToolUsageRecord
	lastThought       *aibrecorder.ModelThoughtRecord
	prompts           int
	tools             int
	thoughts          int
	interceptionError error
	clientIP          string
}

func (r *recordingPrivacyRecorder) RecordInterception(ctx context.Context, req *aibrecorder.InterceptionRecord) error {
	r.interception = req
	r.clientIP = clientip.FromContext(ctx)
	return r.interceptionError
}

func (*recordingPrivacyRecorder) RecordInterceptionEnded(context.Context, *aibrecorder.InterceptionRecordEnded) error {
	return nil
}

func (r *recordingPrivacyRecorder) RecordTokenUsage(_ context.Context, req *aibrecorder.TokenUsageRecord) error {
	r.tokens = req
	return nil
}

func (r *recordingPrivacyRecorder) RecordPromptUsage(_ context.Context, req *aibrecorder.PromptUsageRecord) error {
	r.prompts++
	r.lastPrompt = req
	return nil
}

func (r *recordingPrivacyRecorder) RecordToolUsage(_ context.Context, req *aibrecorder.ToolUsageRecord) error {
	r.tools++
	r.lastTool = req
	return nil
}

func (r *recordingPrivacyRecorder) RecordModelThought(_ context.Context, req *aibrecorder.ModelThoughtRecord) error {
	r.thoughts++
	r.lastThought = req
	return nil
}
