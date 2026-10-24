package proxy

import (
	"context"

	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/requestprivacy"

	aibrecorder "github.com/coder/aibridge/recorder"
)

// PrivacyRecorder enforces request-scoped recording policy before data reaches
// telemetry or persistent recorders. Metadata-only requests retain accounting
// records and token counts while dropping user content and identifying client
// metadata.
type PrivacyRecorder struct {
	inner aibrecorder.Recorder
}

// NewPrivacyRecorder wraps a recorder with request-scoped content filtering.
func NewPrivacyRecorder(inner aibrecorder.Recorder) *PrivacyRecorder {
	return &PrivacyRecorder{inner: inner}
}

func (r *PrivacyRecorder) RecordInterception(ctx context.Context, req *aibrecorder.InterceptionRecord) error {
	if !requestprivacy.IsMetadataOnly(ctx) || req == nil {
		return r.inner.RecordInterception(ctx, req)
	}
	sanitized := *req
	sanitized.Metadata = nil
	sanitized.ClientSessionID = nil
	sanitized.Client = ""
	sanitized.UserAgent = ""
	sanitized.CorrelatingToolCallID = nil
	sanitized.CredentialKind = "model_lab_ticket"
	sanitized.CredentialHint = ""
	return r.inner.RecordInterception(clientip.ContextWithClientIP(ctx, ""), &sanitized)
}

func (r *PrivacyRecorder) RecordInterceptionEnded(ctx context.Context, req *aibrecorder.InterceptionRecordEnded) error {
	return r.inner.RecordInterceptionEnded(ctx, req)
}

func (r *PrivacyRecorder) RecordTokenUsage(ctx context.Context, req *aibrecorder.TokenUsageRecord) error {
	if !requestprivacy.IsMetadataOnly(ctx) || req == nil {
		return r.inner.RecordTokenUsage(ctx, req)
	}
	sanitized := *req
	sanitized.MsgID = ""
	sanitized.Metadata = nil
	return r.inner.RecordTokenUsage(ctx, &sanitized)
}

func (r *PrivacyRecorder) RecordPromptUsage(ctx context.Context, req *aibrecorder.PromptUsageRecord) error {
	if requestprivacy.IsMetadataOnly(ctx) {
		return nil
	}
	return r.inner.RecordPromptUsage(ctx, req)
}

func (r *PrivacyRecorder) RecordToolUsage(ctx context.Context, req *aibrecorder.ToolUsageRecord) error {
	if requestprivacy.IsMetadataOnly(ctx) {
		return nil
	}
	return r.inner.RecordToolUsage(ctx, req)
}

func (r *PrivacyRecorder) RecordModelThought(ctx context.Context, req *aibrecorder.ModelThoughtRecord) error {
	if requestprivacy.IsMetadataOnly(ctx) {
		return nil
	}
	return r.inner.RecordModelThought(ctx, req)
}
