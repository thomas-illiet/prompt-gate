package requestprivacy

import (
	"context"
	"testing"

	cdrslog "cdr.dev/slog/v3"
)

func TestMetadataOnlySinkDropsSensitiveBridgeLogs(t *testing.T) {
	inner := &recordingSink{}
	sink := NewMetadataOnlySink(inner)
	entry := cdrslog.SinkEntry{
		Message: "provider error containing canary-secret",
		Fields:  cdrslog.M(cdrslog.F("payload", "canary-secret")),
	}

	sink.LogEntry(MarkMetadataOnly(context.Background()), entry)
	if len(inner.entries) != 0 {
		t.Fatalf("metadata-only log reached inner sink: %#v", inner.entries)
	}

	sink.LogEntry(context.Background(), entry)
	sink.Sync()
	if len(inner.entries) != 1 || inner.syncs != 1 {
		t.Fatalf("standard log was not delegated: entries=%d syncs=%d", len(inner.entries), inner.syncs)
	}
}

type recordingSink struct {
	entries []cdrslog.SinkEntry
	syncs   int
}

func (s *recordingSink) LogEntry(_ context.Context, entry cdrslog.SinkEntry) {
	s.entries = append(s.entries, entry)
}

func (s *recordingSink) Sync() { s.syncs++ }
