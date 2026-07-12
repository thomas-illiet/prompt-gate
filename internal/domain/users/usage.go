package users

import (
	"context"
	"fmt"
)

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
