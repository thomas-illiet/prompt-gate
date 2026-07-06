package firewall

import (
	"context"
	"fmt"
	"strings"

	"promptgate/backend/internal/platform/configevents"

	"gorm.io/gorm"
)

// ListServiceAccountRulesPaged returns scoped firewall rules with pagination and sorting.
func (s *Service) ListServiceAccountRulesPaged(ctx context.Context, serviceAccountID string, params ListParams) (ListResult, error) {
	return s.listScopedRulesPaged(ctx, RuleTypeServiceAccount, serviceAccountID, params)
}

// ListUserRulesPaged returns user-scoped firewall rules with pagination and sorting.
func (s *Service) ListUserRulesPaged(ctx context.Context, userID string, params ListParams) (ListResult, error) {
	return s.listScopedRulesPaged(ctx, RuleTypeUser, userID, params)
}

// listScopedRulesPaged returns scoped firewall rules with pagination and sorting.
func (s *Service) listScopedRulesPaged(ctx context.Context, ruleType RuleType, referentielID string, params ListParams) (ListResult, error) {
	normalizeListParams(&params)

	query := scopedRuleQuery(s.db.WithContext(ctx).Model(&FirewallRule{}), ruleType, referentielID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListResult{}, fmt.Errorf("count scoped firewall rules: %w", err)
	}

	var records []FirewallRule
	var err error
	query, err = applyFirewallSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return ListResult{}, err
	}
	if err := query.
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&records).Error; err != nil {
		return ListResult{}, fmt.Errorf("list scoped firewall rules: %w", err)
	}

	responses := make([]RuleResponse, len(records))
	for i, record := range records {
		responses[i] = record.toResponse()
	}
	return ListResult{
		Items:    responses,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// GetServiceAccountRule returns one scoped firewall rule by id.
func (s *Service) GetServiceAccountRule(ctx context.Context, serviceAccountID string, id string) (RuleResponse, error) {
	return s.getScopedRuleResponse(ctx, RuleTypeServiceAccount, serviceAccountID, id)
}

// GetUserRule returns one user-scoped firewall rule by id.
func (s *Service) GetUserRule(ctx context.Context, userID string, id string) (RuleResponse, error) {
	return s.getScopedRuleResponse(ctx, RuleTypeUser, userID, id)
}

// getScopedRuleResponse returns one scoped firewall rule by id.
func (s *Service) getScopedRuleResponse(ctx context.Context, ruleType RuleType, referentielID string, id string) (RuleResponse, error) {
	record, err := s.getScopedRule(ctx, s.db, ruleType, referentielID, id)
	if err != nil {
		return RuleResponse{}, err
	}
	return record.toResponse(), nil
}

// CreateServiceAccountRule validates and stores a service-account scoped firewall rule.
func (s *Service) CreateServiceAccountRule(ctx context.Context, serviceAccountID string, input CreateRuleInput) (RuleResponse, error) {
	return s.createScopedRule(ctx, RuleTypeServiceAccount, serviceAccountID, input)
}

// CreateUserRule validates and stores a user-scoped firewall rule.
func (s *Service) CreateUserRule(ctx context.Context, userID string, input CreateRuleInput) (RuleResponse, error) {
	return s.createScopedRule(ctx, RuleTypeUser, userID, input)
}

// createScopedRule validates and stores a scoped firewall rule.
func (s *Service) createScopedRule(ctx context.Context, ruleType RuleType, referentielID string, input CreateRuleInput) (RuleResponse, error) {
	address, err := validateAddress(input.Address)
	if err != nil {
		return RuleResponse{}, err
	}
	if err := validatePriority(input.Priority); err != nil {
		return RuleResponse{}, err
	}
	if err := validateAction(input.Action); err != nil {
		return RuleResponse{}, err
	}

	var total int64
	if err := s.db.WithContext(ctx).
		Model(&FirewallRule{}).
		Where("type = ? AND referentiel_id = ? AND priority = ?", ruleType, referentielID, input.Priority).
		Count(&total).Error; err != nil {
		return RuleResponse{}, fmt.Errorf("check scoped priority conflict: %w", err)
	}
	if total > 0 {
		return RuleResponse{}, ErrPriorityConflict
	}

	record := FirewallRule{
		Type:          ruleType,
		ReferentielID: &referentielID,
		Address:       address,
		Description:   strings.TrimSpace(input.Description),
		Priority:      input.Priority,
		Action:        input.Action,
		Enabled:       input.Enabled,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return RuleResponse{}, ErrPriorityConflict
		}
		return RuleResponse{}, fmt.Errorf("create scoped firewall rule: %w", err)
	}

	s.notifier.Notify(ctx, configevents.DomainFirewall)
	return record.toResponse(), nil
}

// UpdateServiceAccountRule patches a scoped firewall rule and reconciles priority changes.
func (s *Service) UpdateServiceAccountRule(ctx context.Context, serviceAccountID string, id string, input UpdateRuleInput) (RuleResponse, error) {
	return s.updateScopedRule(ctx, RuleTypeServiceAccount, serviceAccountID, id, input)
}

// UpdateUserRule patches a user-scoped firewall rule and reconciles priority changes.
func (s *Service) UpdateUserRule(ctx context.Context, userID string, id string, input UpdateRuleInput) (RuleResponse, error) {
	return s.updateScopedRule(ctx, RuleTypeUser, userID, id, input)
}

// updateScopedRule patches a scoped firewall rule and reconciles priority changes.
func (s *Service) updateScopedRule(ctx context.Context, ruleType RuleType, referentielID string, id string, input UpdateRuleInput) (RuleResponse, error) {
	var address string
	if input.Address != nil {
		parsed, err := validateAddress(*input.Address)
		if err != nil {
			return RuleResponse{}, err
		}
		address = parsed
	}
	if input.Priority != nil {
		if err := validatePriority(*input.Priority); err != nil {
			return RuleResponse{}, err
		}
	}
	if input.Action != nil {
		if err := validateAction(*input.Action); err != nil {
			return RuleResponse{}, err
		}
	}

	var updated FirewallRule
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, err := s.getScopedRule(ctx, tx, ruleType, referentielID, id)
		if err != nil {
			return err
		}

		if input.Address != nil {
			record.Address = address
		}
		if input.Action != nil {
			record.Action = *input.Action
		}
		if input.Description != nil {
			record.Description = strings.TrimSpace(*input.Description)
		}
		if input.Enabled != nil {
			record.Enabled = *input.Enabled
		}

		if input.Priority == nil || *input.Priority == record.Priority {
			updated = record
			return tx.Save(&updated).Error
		}

		if err := s.swapOrSetPriority(ctx, tx, &record, *input.Priority); err != nil {
			return err
		}

		updated = record
		return tx.Save(&updated).Error
	})
	if err != nil {
		return RuleResponse{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainFirewall)
	return updated.toResponse(), nil
}

// MoveServiceAccountPriority swaps a scoped firewall rule with its neighboring priority.
func (s *Service) MoveServiceAccountPriority(ctx context.Context, serviceAccountID string, id string, direction string) (RuleResponse, error) {
	return s.moveScopedPriority(ctx, RuleTypeServiceAccount, serviceAccountID, id, direction)
}

// MoveUserPriority swaps a user-scoped firewall rule with its neighboring priority.
func (s *Service) MoveUserPriority(ctx context.Context, userID string, id string, direction string) (RuleResponse, error) {
	return s.moveScopedPriority(ctx, RuleTypeUser, userID, id, direction)
}

// moveScopedPriority swaps a scoped firewall rule with its neighboring priority.
func (s *Service) moveScopedPriority(ctx context.Context, ruleType RuleType, referentielID string, id string, direction string) (RuleResponse, error) {
	var updated FirewallRule
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, err := s.getScopedRule(ctx, tx, ruleType, referentielID, id)
		if err != nil {
			return err
		}

		target := record.Priority
		switch direction {
		case "increase":
			target++
		case "decrease":
			target--
		default:
			return ErrInvalidDirection
		}

		if err := validatePriority(target); err != nil {
			return err
		}
		if err := s.swapOrSetPriority(ctx, tx, &record, target); err != nil {
			return err
		}

		updated = record
		return tx.Save(&updated).Error
	})
	if err != nil {
		return RuleResponse{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainFirewall)
	return updated.toResponse(), nil
}

// DeleteServiceAccountRule deletes a scoped firewall rule by id.
func (s *Service) DeleteServiceAccountRule(ctx context.Context, serviceAccountID string, id string) error {
	return s.deleteScopedRule(ctx, RuleTypeServiceAccount, serviceAccountID, id)
}

// DeleteUserRule deletes a user-scoped firewall rule by id.
func (s *Service) DeleteUserRule(ctx context.Context, userID string, id string) error {
	return s.deleteScopedRule(ctx, RuleTypeUser, userID, id)
}

// deleteScopedRule deletes a scoped firewall rule by id.
func (s *Service) deleteScopedRule(ctx context.Context, ruleType RuleType, referentielID string, id string) error {
	result := s.db.WithContext(ctx).
		Where("type = ? AND referentiel_id = ?", ruleType, referentielID).
		Delete(&FirewallRule{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete scoped firewall rule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRuleNotFound
	}
	s.notifier.Notify(ctx, configevents.DomainFirewall)
	return nil
}
