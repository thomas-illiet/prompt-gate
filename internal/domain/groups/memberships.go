package groups

import (
	"context"
	"fmt"
	"strings"

	"promptgate/backend/internal/platform/configevents"

	"gorm.io/gorm"
)

func (s *Service) AddMember(ctx context.Context, groupIDRaw, userID string) error {
	groupID, err := parseGroupID(groupIDRaw)
	if err != nil {
		return ErrGroupNotFound
	}
	userID = strings.TrimSpace(userID)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.ensureGroupExists(ctx, tx, groupID); err != nil {
			return err
		}
		if err := s.ensureUserExists(ctx, tx, userID); err != nil {
			return err
		}
		member := GroupMember{GroupID: groupID, UserID: userID}
		if err := tx.WithContext(ctx).FirstOrCreate(&member, "group_id = ? AND user_id = ?", groupID, userID).Error; err != nil {
			return fmt.Errorf("add group member: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.notifier.Notify(ctx, configevents.DomainGroups)
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, groupIDRaw, userID string) error {
	groupID, err := parseGroupID(groupIDRaw)
	if err != nil {
		return ErrGroupNotFound
	}
	userID = strings.TrimSpace(userID)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.ensureGroupExists(ctx, tx, groupID); err != nil {
			return err
		}
		if err := s.ensureUserExists(ctx, tx, userID); err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&GroupMember{}).Error; err != nil {
			return fmt.Errorf("remove group member: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.notifier.Notify(ctx, configevents.DomainGroups)
	return nil
}

func (s *Service) ListUserGroups(ctx context.Context, userID string) ([]GroupResponse, error) {
	userID = strings.TrimSpace(userID)
	if err := s.ensureUserExists(ctx, s.db, userID); err != nil {
		return nil, err
	}
	var records []Group
	if err := s.db.WithContext(ctx).
		Joins("JOIN access_group_members ON access_group_members.group_id = access_groups.id").
		Where("access_group_members.user_id = ?", userID).
		Preload("Providers").Preload("ModelPatterns").Preload("Members").
		Order("access_groups.name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list user groups: %w", err)
	}
	out := make([]GroupResponse, len(records))
	for i := range records {
		out[i] = records[i].toResponse()
	}
	return out, nil
}

func (s *Service) ListUserGroupSummaries(ctx context.Context, userID string) ([]ProfileGroupResponse, error) {
	userID = strings.TrimSpace(userID)
	if err := s.ensureUserExists(ctx, s.db, userID); err != nil {
		return nil, err
	}
	var records []Group
	if err := s.db.WithContext(ctx).
		Select("access_groups.id", "access_groups.name", "access_groups.display_name", "access_groups.description").
		Joins("JOIN access_group_members ON access_group_members.group_id = access_groups.id").
		Where("access_group_members.user_id = ?", userID).
		Order("access_groups.name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list user group summaries: %w", err)
	}
	out := make([]ProfileGroupResponse, len(records))
	for i := range records {
		out[i] = records[i].toProfileResponse()
	}
	return out, nil
}

func (s *Service) ReplaceUserGroups(ctx context.Context, userID string, groupIDsRaw []string) ([]GroupResponse, error) {
	userID = strings.TrimSpace(userID)
	groupIDs, err := parseGroupIDs(groupIDsRaw)
	if err != nil {
		return nil, ErrGroupNotFound
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.ensureUserExists(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.ensureGroupsExist(ctx, tx, groupIDs); err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("user_id = ?", userID).Delete(&GroupMember{}).Error; err != nil {
			return fmt.Errorf("delete user group memberships: %w", err)
		}
		for _, groupID := range groupIDs {
			if err := tx.WithContext(ctx).Create(&GroupMember{GroupID: groupID, UserID: userID}).Error; err != nil {
				return fmt.Errorf("create user group membership: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notifier.Notify(ctx, configevents.DomainGroups)
	return s.ListUserGroups(ctx, userID)
}
