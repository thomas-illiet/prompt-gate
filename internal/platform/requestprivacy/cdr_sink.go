package requestprivacy

import (
	"context"

	cdrslog "cdr.dev/slog/v3"
)

// MetadataOnlySink drops bridge diagnostics for metadata-only requests. AI
// bridge errors can include provider events or request-derived values, so
// field-level redaction is not sufficient for this privacy boundary.
type MetadataOnlySink struct {
	inner cdrslog.Sink
}

// NewMetadataOnlySink wraps a CDR slog sink with the Model Lab privacy policy.
func NewMetadataOnlySink(inner cdrslog.Sink) *MetadataOnlySink {
	return &MetadataOnlySink{inner: inner}
}

func (s *MetadataOnlySink) LogEntry(ctx context.Context, entry cdrslog.SinkEntry) {
	if s == nil || s.inner == nil || IsMetadataOnly(ctx) {
		return
	}
	s.inner.LogEntry(ctx, entry)
}

func (s *MetadataOnlySink) Sync() {
	if s == nil || s.inner == nil {
		return
	}
	s.inner.Sync()
}
