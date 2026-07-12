package monitoring

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// CheckService runs one immediate HTTP check and persists the result.
func (s *Service) CheckService(ctx context.Context, id string) (ServiceResponse, error) {
	record, err := s.getService(ctx, s.db, id)
	if err != nil {
		return ServiceResponse{}, err
	}
	checked, err := s.checkRecord(ctx, record)
	if err != nil {
		return ServiceResponse{}, err
	}
	return checked.toResponse(), nil
}

// RunDueChecks checks every enabled service whose per-service interval has elapsed.
func (s *Service) RunDueChecks(ctx context.Context, now time.Time) (int, error) {
	records, err := s.ListEnabledDue(ctx, now)
	if err != nil {
		return 0, err
	}
	for _, record := range records {
		if _, err := s.checkRecord(ctx, record); err != nil {
			return 0, err
		}
	}
	return len(records), nil
}

// ListEnabledDue returns enabled services whose interval has elapsed.
func (s *Service) ListEnabledDue(ctx context.Context, now time.Time) ([]MonitoringService, error) {
	now = now.UTC()
	var records []MonitoringService
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("name ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list due monitoring services: %w", err)
	}

	out := make([]MonitoringService, 0, len(records))
	for _, record := range records {
		interval := time.Duration(record.intervalSeconds()) * time.Second
		if record.LastCheckedAt == nil || !record.LastCheckedAt.Add(interval).After(now) {
			out = append(out, record)
		}
	}
	return out, nil
}

// CurrentStatus returns currently degraded enabled monitoring services for app users.
func (s *Service) CurrentStatus(ctx context.Context) (StatusResponse, error) {
	var records []MonitoringService
	if err := s.db.WithContext(ctx).
		Where("enabled = ? AND status = ?", true, StatusDegraded).
		Order("name ASC").
		Find(&records).Error; err != nil {
		return StatusResponse{}, fmt.Errorf("load monitoring status: %w", err)
	}

	out := StatusResponse{Status: StatusOK, Services: []StatusServiceResponse{}}
	if len(records) > 0 {
		out.Status = StatusDegraded
	}
	for _, record := range records {
		out.Services = append(out.Services, StatusServiceResponse{
			ID:             record.ID,
			Name:           record.Name,
			DisplayName:    record.DisplayName,
			Status:         record.Status,
			LastCheckedAt:  record.LastCheckedAt,
			LastStatusCode: record.LastStatusCode,
			LastError:      record.LastError,
			LastLatencyMS:  record.LastLatencyMS,
		})
	}
	return out, nil
}

// StartScheduler starts a background monitoring checker tied to the context lifetime.
func (s *Service) StartScheduler(ctx context.Context, tick time.Duration) {
	if tick <= 0 {
		tick = DefaultSchedulerTick
	}

	go func() {
		s.runDueChecksLog(ctx)
		ticker := time.NewTicker(tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runDueChecksLog(ctx)
			}
		}
	}()
}

func (s *Service) checkRecord(ctx context.Context, record MonitoringService) (MonitoringService, error) {
	startedAt := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, record.URL, nil)
	if err != nil {
		return MonitoringService{}, fmt.Errorf("build monitoring request: %w", err)
	}

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: DefaultCheckTimeout}
	}
	resp, err := client.Do(req)
	latencyMS := time.Since(startedAt).Milliseconds()
	checkedAt := time.Now().UTC()

	record.LastCheckedAt = &checkedAt
	record.LastLatencyMS = latencyMS
	record.LastStatusCode = nil
	record.LastError = ""

	if err != nil {
		record.Status = StatusDegraded
		record.LastError = truncateError(err.Error())
		record.ConsecutiveFailures++
		return record, s.saveCheckResult(ctx, record)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	statusCode := resp.StatusCode
	record.LastStatusCode = &statusCode
	if statusCode == record.ExpectedStatusCode {
		record.Status = StatusOK
		record.ConsecutiveFailures = 0
	} else {
		record.Status = StatusDegraded
		record.LastError = fmt.Sprintf("expected HTTP %d, got %d", record.ExpectedStatusCode, statusCode)
		record.ConsecutiveFailures++
	}
	return record, s.saveCheckResult(ctx, record)
}

func (s *Service) saveCheckResult(ctx context.Context, record MonitoringService) error {
	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return fmt.Errorf("save monitoring check result: %w", err)
	}
	return nil
}

func (s *Service) runDueChecksLog(ctx context.Context) {
	count, err := s.RunDueChecks(ctx, time.Now().UTC())
	if err != nil {
		slog.Error("failed to run monitoring checks", "error", err)
		return
	}
	if count > 0 {
		slog.Info("monitoring checks completed", "services", count)
	}
}
