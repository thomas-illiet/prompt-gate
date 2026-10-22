package users

import (
	"context"
	"fmt"
	"time"
)

const activeUserWindowDays = 30

func (s *Service) attachAdminUserUsage(ctx context.Context, items []AdminUser) error {
	if err := s.attachAdminUserTokenConsumption(ctx, items); err != nil {
		return err
	}
	return s.attachAdminUserActivity(ctx, items)
}

func (s *Service) attachAdminUserTokenConsumption(ctx context.Context, items []AdminUser) error {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	consumptionByUserID, err := s.loadTokenConsumption(ctx, ids)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].InputTokens = consumptionByUserID[items[i].ID].InputTokens
		items[i].OutputTokens = consumptionByUserID[items[i].ID].OutputTokens
	}
	return nil
}

func (s *Service) attachAdminUserActivity(ctx context.Context, items []AdminUser) error {
	if len(items) == 0 {
		return nil
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	today := utcDay(s.nowUTC())
	startsAt := today.AddDate(0, 0, -(activeUserWindowDays - 1))
	var activeIDs []string
	if err := s.db.WithContext(ctx).
		Table("proxy_daily_usage_kpis").
		Where("initiator_id IN ? AND day >= ? AND day <= ? AND requests > 0", ids, startsAt, today).
		Distinct().
		Pluck("initiator_id", &activeIDs).Error; err != nil {
		return fmt.Errorf("load recent user activity: %w", err)
	}

	activeByID := make(map[string]struct{}, len(activeIDs))
	for _, id := range activeIDs {
		activeByID[id] = struct{}{}
	}
	for i := range items {
		_, items[i].ActiveInLast30Days = activeByID[items[i].ID]
	}
	return nil
}

func utcDay(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *Service) attachServiceAccountTokenConsumption(ctx context.Context, items []ServiceAccount) error {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	consumptionByUserID, err := s.loadTokenConsumption(ctx, ids)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].InputTokens = consumptionByUserID[items[i].ID].InputTokens
		items[i].OutputTokens = consumptionByUserID[items[i].ID].OutputTokens
	}
	return nil
}

func (s *Service) loadTokenConsumption(ctx context.Context, userIDs []string) (map[string]tokenConsumption, error) {
	if len(userIDs) == 0 {
		return map[string]tokenConsumption{}, nil
	}
	var rows []tokenConsumption
	if err := s.db.WithContext(ctx).
		Table("token_usages").
		Select(`interceptions.initiator_id AS user_id,
			COALESCE(SUM(token_usages.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(token_usages.output_tokens), 0) AS output_tokens`).
		Joins("JOIN interceptions ON interceptions.id = token_usages.interception_id").
		Where("interceptions.initiator_id IN ?", userIDs).
		Group("interceptions.initiator_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load token consumption: %w", err)
	}
	consumptionByUserID := make(map[string]tokenConsumption, len(rows))
	for _, row := range rows {
		consumptionByUserID[row.UserID] = row
	}
	return consumptionByUserID, nil
}
