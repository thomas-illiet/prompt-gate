package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type quotaProjection struct {
	UserID        string `gorm:"column:user_id"`
	Used5HTokens  int64  `gorm:"column:used_5h_tokens"`
	Quota5HTokens *int64 `gorm:"column:quota_5h_tokens"`
	Used7DTokens  int64  `gorm:"column:used_7d_tokens"`
	Quota7DTokens *int64 `gorm:"column:quota_7d_tokens"`
}

// ScanQuotaExhaustion emits alerts only on a fresh non-exhausted to exhausted transition.
func (s *Service) ScanQuotaExhaustion(ctx context.Context, now time.Time) (int, error) {
	if !s.Enabled() {
		return 0, nil
	}
	var rows []quotaProjection
	if err := s.db.WithContext(ctx).Table("subscription_quota_states q").
		Select("q.user_id, q.used_5h_tokens, q.quota_5h_tokens, q.used_7d_tokens, q.quota_7d_tokens").
		Joins("JOIN users u ON u.id = q.user_id").
		Where("u.type = ?", "user").Find(&rows).Error; err != nil {
		return 0, err
	}
	emitted := 0
	for _, row := range rows {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var state QuotaState
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", row.UserID).Take(&state).Error
			current5 := row.Quota5HTokens != nil && *row.Quota5HTokens > 0 && row.Used5HTokens >= *row.Quota5HTokens
			current7 := row.Quota7DTokens != nil && *row.Quota7DTokens > 0 && row.Used7DTokens >= *row.Quota7DTokens
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Create(&QuotaState{UserID: row.UserID, Exhausted5H: current5, Exhausted7D: current7}).Error
			}
			if err != nil {
				return err
			}
			fresh5, fresh7 := current5 && !state.Exhausted5H, current7 && !state.Exhausted7D
			state.Exhausted5H, state.Exhausted7D = current5, current7
			if !fresh5 && !fresh7 {
				return tx.Save(&state).Error
			}
			state.Sequence++
			if err := tx.Save(&state).Error; err != nil {
				return err
			}
			windows := make([]string, 0, 2)
			if fresh5 {
				windows = append(windows, "5-hour")
			}
			if fresh7 {
				windows = append(windows, "7-day")
			}
			body := fmt.Sprintf("# Quota exhausted\n\n**Affected window:** %s.\n\n**Your quickest route back:** use an available reset credit. Otherwise, requests resume automatically when usage leaves the rolling window.\n\n**Does this look unexpected?** Contact an administrator and we will help you check it.", strings.Join(windows, " and "))
			if err := s.EnqueueAutomaticTx(ctx, tx, KindQuotaExhausted, row.UserID, fmt.Sprintf("quota-exhausted:%s:%d", row.UserID, state.Sequence), "Snack break: quota reached 100%", body); err != nil {
				return err
			}
			emitted++
			return nil
		})
		if err != nil {
			return emitted, err
		}
	}
	return emitted, nil
}

type monthlyTotals struct {
	Requests              int64
	TotalDurationMs       int64
	InputTokens           int64
	OutputTokens          int64
	CacheReadInputTokens  int64
	CacheWriteInputTokens int64
	EmbeddingTokens       int64
	TotalTokens           int64
	ActiveDays            int64
}

type namedUsage struct {
	Name        string
	TotalTokens int64
}

// GenerateMonthlyReports creates idempotent reports after 08:00 on the first
// local day of a month, or later as a catch-up if that window was missed.
func (s *Service) GenerateMonthlyReports(ctx context.Context, now time.Time, location *time.Location) (int, error) {
	if !s.Enabled() {
		return 0, nil
	}
	if location == nil {
		location = time.UTC
	}
	localNow := now.In(location)
	if localNow.Day() == 1 && localNow.Hour() < 8 {
		return 0, nil
	}
	endLocal := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, location)
	startLocal := endLocal.AddDate(0, -1, 0)
	startDay, endDay := startLocal.Format("2006-01-02"), endLocal.Format("2006-01-02")
	var recipients []recipientRow
	if err := s.db.WithContext(ctx).Table("users u").Select("u.id, u.email, u.name, u.is_active, u.role, u.expires_at").
		Where("u.type = ? AND u.is_active = ? AND u.role <> ? AND (u.expires_at IS NULL OR u.expires_at > ?)", "user", true, "none", now.UTC()).
		Where("EXISTS (SELECT 1 FROM proxy_daily_usage_kpis k WHERE k.initiator_id = u.id AND k.day >= ? AND k.day < ? AND k.requests > 0)", startDay, endDay).
		Find(&recipients).Error; err != nil {
		return 0, err
	}
	created := 0
	for _, recipient := range recipients {
		if _, err := normalizeAddress(recipient.Email); err != nil {
			continue
		}
		var totals monthlyTotals
		if err := s.db.WithContext(ctx).Table("proxy_daily_usage_kpis").Select("COALESCE(SUM(requests),0) requests, COALESCE(SUM(total_duration_ms),0) total_duration_ms, COALESCE(SUM(input_tokens),0) input_tokens, COALESCE(SUM(output_tokens),0) output_tokens, COALESCE(SUM(cache_read_input_tokens),0) cache_read_input_tokens, COALESCE(SUM(cache_write_input_tokens),0) cache_write_input_tokens, COALESCE(SUM(embedding_tokens),0) embedding_tokens, COALESCE(SUM(total_tokens),0) total_tokens, COUNT(*) active_days").Where("initiator_id = ? AND day >= ? AND day < ?", recipient.ID, startDay, endDay).Scan(&totals).Error; err != nil {
			return created, err
		}
		models, err := s.topMonthlyUsage(ctx, recipient.ID, startDay, endDay, "models")
		if err != nil {
			return created, err
		}
		providers, err := s.topMonthlyUsage(ctx, recipient.ID, startDay, endDay, "provider-names")
		if err != nil {
			return created, err
		}
		cost := (float64(totals.InputTokens)*s.inputRate + float64(totals.OutputTokens)*s.outputRate + float64(totals.EmbeddingTokens)*s.embeddingRate) / 1_000_000
		latency := float64(0)
		if totals.Requests > 0 {
			latency = float64(totals.TotalDurationMs) / float64(totals.Requests)
		}
		body := fmt.Sprintf("# Your %s usage report\n\n**Requests:** %d across **%d active days**.\n\n**Total tokens:** %d — %d input, %d output, and %d embedding.\n\n**Cache activity:** %d read tokens and %d write tokens.\n\n**Performance:** %.0f ms average latency, with an estimated cost of **$%.4f USD** for the complete retained period.\n\n**Most used models:** %s.\n\n**Most used providers:** %s.%s", startLocal.Format("January 2006"), totals.Requests, totals.ActiveDays, totals.TotalTokens, totals.InputTokens, totals.OutputTokens, totals.EmbeddingTokens, totals.CacheReadInputTokens, totals.CacheWriteInputTokens, latency, cost, formatNamedUsage(models), formatNamedUsage(providers), s.currentQuotaSummary(ctx, recipient.ID))
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return s.EnqueueAutomaticTx(ctx, tx, KindMonthlyReport, recipient.ID, "monthly-report:"+recipient.ID+":"+startLocal.Format("2006-01"), "Your month in prompts — "+startLocal.Format("January 2006"), body)
		})
		if err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

func (s *Service) topMonthlyUsage(ctx context.Context, userID, startDay, endDay, dimension string) ([]namedUsage, error) {
	var rows []namedUsage
	err := s.db.WithContext(ctx).Table("proxy_daily_usage_breakdowns").Select("name, SUM(total_tokens) total_tokens").Where("initiator_id = ? AND day >= ? AND day < ? AND dimension = ?", userID, startDay, endDay, dimension).Group("name").Order("total_tokens DESC").Limit(5).Scan(&rows).Error
	return rows, err
}

func formatNamedUsage(rows []namedUsage) string {
	if len(rows) == 0 {
		return "None"
	}
	values := make([]string, len(rows))
	for i := range rows {
		values[i] = fmt.Sprintf("%s (%d)", rows[i].Name, rows[i].TotalTokens)
	}
	return strings.Join(values, ", ")
}

func (s *Service) currentQuotaSummary(ctx context.Context, userID string) string {
	var row quotaProjection
	if err := s.db.WithContext(ctx).Table("subscription_quota_states").Select("user_id, used_5h_tokens, quota_5h_tokens, used_7d_tokens, quota_7d_tokens").Where("user_id = ?", userID).Take(&row).Error; err != nil {
		return ""
	}
	format := func(used int64, quota *int64) string {
		if quota == nil {
			return "unlimited"
		}
		return fmt.Sprintf("%d / %d tokens", used, *quota)
	}
	return fmt.Sprintf("\n\n## Current quota state\n\n**5-hour window:** %s\n\n**7-day window:** %s", format(row.Used5HTokens, row.Quota5HTokens), format(row.Used7DTokens, row.Quota7DTokens))
}
