package users

import (
	"context"
	"fmt"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/configevents"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) ListServiceAccounts(ctx context.Context) ([]ServiceAccount, error) {
	result, err := s.ListServiceAccountsPaged(ctx, ServiceAccountListParams{
		Page:     1,
		PageSize: 100,
		SortBy:   "createdAt",
		SortDir:  "desc",
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *Service) ListServiceAccountsPaged(ctx context.Context, params ServiceAccountListParams) (ServiceAccountListResult, error) {
	normalizeServiceAccountListParams(&params)
	query := s.db.WithContext(ctx).Model(&User{}).Where("type = ?", auth.UserTypeService)
	if userSortNeedsConsumption(params.SortBy) {
		query = query.Joins(userTokenConsumptionJoin())
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ServiceAccountListResult{}, fmt.Errorf("count service accounts: %w", err)
	}
	query, err := applyServiceAccountSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return ServiceAccountListResult{}, err
	}
	var records []User
	if err := query.Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).Find(&records).Error; err != nil {
		return ServiceAccountListResult{}, fmt.Errorf("list service accounts: %w", err)
	}
	items := make([]ServiceAccount, 0, len(records))
	for _, record := range records {
		items = append(items, record.serviceAccount())
	}
	if err := s.attachServiceAccountTokenConsumption(ctx, items); err != nil {
		return ServiceAccountListResult{}, err
	}
	return ServiceAccountListResult{
		Items:    items,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

func (s *Service) GetServiceAccount(ctx context.Context, id string) (ServiceAccount, error) {
	record, err := s.findServiceAccount(ctx, s.db, id)
	if err != nil {
		return ServiceAccount{}, err
	}
	items := []ServiceAccount{record.serviceAccount()}
	if err := s.attachServiceAccountTokenConsumption(ctx, items); err != nil {
		return ServiceAccount{}, err
	}
	return items[0], nil
}

func (s *Service) ServiceAccountProfile(ctx context.Context, id string) (auth.UserProfile, error) {
	record, err := s.findServiceAccount(ctx, s.db, id)
	if err != nil {
		return auth.UserProfile{}, err
	}
	return record.profile(), nil
}

func (s *Service) CreateServiceAccount(ctx context.Context, input ServiceAccountInput) (ServiceAccount, error) {
	normalized, name, err := normalizeServiceAccountInput(input)
	if err != nil {
		return ServiceAccount{}, err
	}
	var record User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.ensureServiceAccountIdentifierAvailable(ctx, tx, normalized, ""); err != nil {
			return err
		}
		record = User{
			ExternalSub:             "service:" + uuid.NewString(),
			PreferredUsername:       normalized,
			Name:                    name,
			Type:                    auth.UserTypeService,
			Role:                    auth.RoleUser,
			IsActive:                input.IsActive,
			FirewallOverrideEnabled: serviceAccountFirewallOverride(input, false),
			LastLoginAt:             time.Now().UTC(),
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("create service account: %w", err)
		}
		return nil
	})
	if err != nil {
		return ServiceAccount{}, err
	}
	s.notifier.Notify(ctx, configevents.DomainAuth)
	return record.serviceAccount(), nil
}

func (s *Service) UpdateServiceAccount(ctx context.Context, id string, input ServiceAccountInput) (ServiceAccount, error) {
	normalized, name, err := normalizeServiceAccountInput(input)
	if err != nil {
		return ServiceAccount{}, err
	}
	var record User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		loaded, err := s.findServiceAccount(ctx, tx, id)
		if err != nil {
			return err
		}
		record = loaded
		if err := s.ensureServiceAccountIdentifierAvailable(ctx, tx, normalized, id); err != nil {
			return err
		}
		record.PreferredUsername = normalized
		record.Name = name
		record.Type = auth.UserTypeService
		record.Role = auth.RoleUser
		record.IsActive = input.IsActive
		record.FirewallOverrideEnabled = serviceAccountFirewallOverride(input, record.FirewallOverrideEnabled)
		if err := tx.Save(&record).Error; err != nil {
			return fmt.Errorf("update service account: %w", err)
		}
		return nil
	})
	if err != nil {
		return ServiceAccount{}, err
	}
	s.notifier.Notify(ctx, configevents.DomainAuth)
	items := []ServiceAccount{record.serviceAccount()}
	if err := s.attachServiceAccountTokenConsumption(ctx, items); err != nil {
		return ServiceAccount{}, err
	}
	return items[0], nil
}

func (s *Service) DeleteServiceAccount(ctx context.Context, id string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := deleteAccountFirewallRulesTx(tx, "service_account", id); err != nil {
			return err
		}
		result := tx.Where("type = ?", auth.UserTypeService).Delete(&User{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("delete service account: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrUserNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.notifier.Notify(ctx, configevents.DomainAuth)
	s.notifier.Notify(ctx, configevents.DomainFirewall)
	return nil
}
