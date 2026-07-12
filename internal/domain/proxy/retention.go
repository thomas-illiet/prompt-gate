package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// StartRawUsageCleanup periodically deletes raw proxy usage rows older than retention.
func (s *Service) StartRawUsageCleanup(ctx context.Context, retention, interval time.Duration) {
	if retention <= 0 || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanupRawUsageLog(context.Background(), retention)
			}
		}
	}()
}

// DeleteRawUsageBefore deletes raw usage roots older than cutoff and relies on FK cascade for child rows.
func (s *Service) DeleteRawUsageBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Where("started_at < ?", cutoff.UTC()).Delete(&Interception{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete raw proxy usage: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// DeleteProcessedUsageBefore removes idempotency markers older than cutoff.
// The retention window should exceed the maximum Redis retry/DLQ lifetime.
func (s *Service) DeleteProcessedUsageBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Where("processed_at < ?", cutoff.UTC()).Delete(&ProcessedUsageEvent{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete processed usage markers: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (s *Service) cleanupRawUsageLog(ctx context.Context, retention time.Duration) {
	cutoff := time.Now().UTC().Add(-retention)
	count, err := s.DeleteRawUsageBefore(ctx, cutoff)
	if err != nil {
		slog.Error("failed to cleanup raw proxy usage", "error", err)
		return
	}
	if count > 0 {
		slog.Info("cleaned up raw proxy usage", "interceptions", count, "cutoff", cutoff)
	}
	markerCutoff := cutoff.Add(-24 * time.Hour)
	markers, err := s.DeleteProcessedUsageBefore(ctx, markerCutoff)
	if err != nil {
		slog.Error("failed to cleanup processed usage markers", "error", err)
	} else if markers > 0 {
		slog.Info("cleaned up processed usage markers", "events", markers, "cutoff", markerCutoff)
	}
}
