package firewall

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// normalizeListParams applies default firewall pagination and sorting values.
func normalizeListParams(params *ListParams) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	if params.SortBy == "" {
		params.SortBy = "priority"
	}
	if params.SortDir == "" {
		params.SortDir = "asc"
	}
}

// applyFirewallSort applies a validated firewall order to the query.
func applyFirewallSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"priority":    "priority",
		"address":     "address",
		"description": "description",
		"action":      "action",
		"enabled":     "enabled",
		"createdAt":   "created_at",
		"updatedAt":   "updated_at",
	}

	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	return query.Order(column + " " + dir).Order("id ASC"), nil
}

// normalizeSortDir converts a firewall sort direction into SQL syntax.
func normalizeSortDir(sortDir string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(sortDir)) {
	case "asc":
		return "ASC", nil
	case "desc":
		return "DESC", nil
	default:
		return "", ErrInvalidSort
	}
}

// getRule fetches a firewall rule or returns ErrNotFound.
func (s *Service) getRule(ctx context.Context, db *gorm.DB, id string) (FirewallRule, error) {
	var record FirewallRule
	if err := globalRuleQuery(db.WithContext(ctx)).
		First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FirewallRule{}, ErrRuleNotFound
		}
		return FirewallRule{}, fmt.Errorf("get firewall rule: %w", err)
	}
	return record, nil
}

// getScopedRule fetches a scoped firewall rule or returns ErrNotFound.
func (s *Service) getScopedRule(ctx context.Context, db *gorm.DB, ruleType RuleType, referentielID string, id string) (FirewallRule, error) {
	var record FirewallRule
	if err := db.WithContext(ctx).
		Where("type = ? AND referentiel_id = ?", ruleType, referentielID).
		First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FirewallRule{}, ErrRuleNotFound
		}
		return FirewallRule{}, fmt.Errorf("get scoped firewall rule: %w", err)
	}
	return record, nil
}

// globalRuleQuery scopes a query to global firewall rules.
func globalRuleQuery(query *gorm.DB) *gorm.DB {
	return query.Where("type = ? AND referentiel_id IS NULL", RuleTypeGlobal)
}

// scopedRuleQuery scopes a query to one firewall rule namespace.
func scopedRuleQuery(query *gorm.DB, ruleType RuleType, referentielID string) *gorm.DB {
	return query.Where("type = ? AND referentiel_id = ?", ruleType, referentielID)
}

// scopedPriorityQuery scopes priority lookups to the same firewall rule namespace.
func scopedPriorityQuery(query *gorm.DB, ruleType RuleType, referentielID *string) *gorm.DB {
	if (ruleType == RuleTypeServiceAccount || ruleType == RuleTypeUser) && referentielID != nil {
		return query.Where("type = ? AND referentiel_id = ?", ruleType, *referentielID)
	}
	return globalRuleQuery(query)
}

// swapOrSetPriority moves a rule priority without leaving duplicate priorities.
func (s *Service) swapOrSetPriority(ctx context.Context, tx *gorm.DB, record *FirewallRule, targetPriority int) error {
	var matches []FirewallRule
	if err := scopedPriorityQuery(tx.WithContext(ctx), record.Type, record.ReferentielID).
		Where("priority = ? AND id <> ?", targetPriority, record.ID).
		Limit(1).
		Find(&matches).Error; err != nil {
		return fmt.Errorf("find priority target: %w", err)
	}
	if len(matches) == 0 {
		record.Priority = targetPriority
		return nil
	}
	other := matches[0]

	oldPriority := record.Priority
	tempPriority := -oldPriority
	if err := tx.WithContext(ctx).
		Model(&FirewallRule{}).
		Where("id = ?", record.ID).
		Update("priority", tempPriority).Error; err != nil {
		return fmt.Errorf("reserve temporary priority: %w", err)
	}
	if err := tx.WithContext(ctx).
		Model(&FirewallRule{}).
		Where("id = ?", other.ID).
		Update("priority", oldPriority).Error; err != nil {
		return fmt.Errorf("swap existing priority: %w", err)
	}

	record.Priority = targetPriority
	return nil
}
