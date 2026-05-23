package firewall

import (
	"context"
	"fmt"
	"strings"

	"promptgate/backend/internal/platform/configevents"

	"gorm.io/gorm"
)

// ListRules returns firewall rules ordered by priority.
func (s *Service) ListRules(ctx context.Context) ([]RuleResponse, error) {
	result, err := s.ListRulesPaged(ctx, ListParams{
		Page:     1,
		PageSize: 100,
		SortBy:   "priority",
		SortDir:  "asc",
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ListRulesPaged returns firewall rules with pagination and sorting.
func (s *Service) ListRulesPaged(ctx context.Context, params ListParams) (ListResult, error) {
	normalizeListParams(&params)

	query := globalRuleQuery(s.db.WithContext(ctx).Model(&FirewallRule{}))
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListResult{}, fmt.Errorf("count firewall rules: %w", err)
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
		return ListResult{}, fmt.Errorf("list firewall rules: %w", err)
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

// GetRule returns one firewall rule by id.
func (s *Service) GetRule(ctx context.Context, id string) (RuleResponse, error) {
	record, err := s.getRule(ctx, s.db, id)
	if err != nil {
		return RuleResponse{}, err
	}
	return record.toResponse(), nil
}

// CreateRule validates and stores a firewall rule.
func (s *Service) CreateRule(ctx context.Context, input CreateRuleInput) (RuleResponse, error) {
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
		Where("type = ? AND referentiel_id IS NULL AND priority = ?", RuleTypeGlobal, input.Priority).
		Count(&total).Error; err != nil {
		return RuleResponse{}, fmt.Errorf("check priority conflict: %w", err)
	}
	if total > 0 {
		return RuleResponse{}, ErrPriorityConflict
	}

	record := FirewallRule{
		Type:        RuleTypeGlobal,
		Address:     address,
		Description: strings.TrimSpace(input.Description),
		Priority:    input.Priority,
		Action:      input.Action,
		Enabled:     input.Enabled,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return RuleResponse{}, ErrPriorityConflict
		}
		return RuleResponse{}, fmt.Errorf("create firewall rule: %w", err)
	}

	s.notifier.Notify(ctx, configevents.DomainFirewall)
	return record.toResponse(), nil
}

// UpdateRule patches a firewall rule and reconciles priority changes.
func (s *Service) UpdateRule(ctx context.Context, id string, input UpdateRuleInput) (RuleResponse, error) {
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
		record, err := s.getRule(ctx, tx, id)
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

// MovePriority swaps a firewall rule with its neighboring priority.
func (s *Service) MovePriority(ctx context.Context, id string, direction string) (RuleResponse, error) {
	var updated FirewallRule
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, err := s.getRule(ctx, tx, id)
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

// DeleteRule deletes a firewall rule by id.
func (s *Service) DeleteRule(ctx context.Context, id string) error {
	result := globalRuleQuery(s.db.WithContext(ctx)).Delete(&FirewallRule{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete firewall rule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRuleNotFound
	}
	s.notifier.Notify(ctx, configevents.DomainFirewall)
	return nil
}
