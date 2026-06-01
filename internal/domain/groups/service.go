package groups

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/configevents"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrGroupNotFound      = errors.New("group not found")
	ErrInvalidName        = errors.New("invalid_name")
	ErrInvalidDisplayName = errors.New("invalid_display_name")
	ErrNameConflict       = errors.New("name_conflict")
	ErrInvalidRegex       = errors.New("invalid_regex")
	ErrInvalidSort        = errors.New("invalid_sort")
	ErrProviderRequired   = errors.New("provider_required")
	ErrProviderNotFound   = errors.New("provider not found")
	ErrUserNotFound       = errors.New("user not found")
)

var groupNameRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const defaultAllModelsPattern = ".*"

type Service struct {
	db       *gorm.DB
	notifier configevents.Notifier
}

// NewService creates an access group service backed by GORM.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db, notifier: configevents.NoopNotifier{}}
}

// SetNotifier configures config event publication after group mutations.
func (s *Service) SetNotifier(notifier configevents.Notifier) {
	if notifier == nil {
		notifier = configevents.NoopNotifier{}
	}
	s.notifier = notifier
}

// AutoMigrate migrates access group tables.
func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(
		&Group{},
		&GroupProvider{},
		&GroupModelPattern{},
		&GroupMember{},
	)
}

// ListGroupsPaged returns access groups with pagination, search, sorting, and related counts.
func (s *Service) ListGroupsPaged(ctx context.Context, params ListParams) (ListResult, error) {
	normalizeListParams(&params)

	query := s.db.WithContext(ctx).Model(&Group{})
	if search := strings.TrimSpace(strings.ToLower(params.Search)); search != "" {
		like := "%" + search + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(description) LIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListResult{}, fmt.Errorf("count groups: %w", err)
	}

	var records []Group
	var err error
	query, err = applyGroupSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return ListResult{}, err
	}
	if err := query.
		Preload("Providers").
		Preload("ModelPatterns").
		Preload("Members").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&records).Error; err != nil {
		return ListResult{}, fmt.Errorf("list groups: %w", err)
	}

	items := make([]GroupResponse, len(records))
	for i := range records {
		items[i] = records[i].toResponse()
	}
	return ListResult{
		Items:    items,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// GetGroup returns one access group by ID.
func (s *Service) GetGroup(ctx context.Context, id string) (GroupResponse, error) {
	record, err := s.getGroup(ctx, s.db, id)
	if err != nil {
		return GroupResponse{}, err
	}
	return record.toResponse(), nil
}

// CreateGroup validates and stores a new access group.
func (s *Service) CreateGroup(ctx context.Context, input CreateGroupInput) (GroupResponse, error) {
	name, err := validateName(input.Name)
	if err != nil {
		return GroupResponse{}, err
	}
	displayName, err := validateDisplayName(input.DisplayName)
	if err != nil {
		return GroupResponse{}, err
	}
	providerIDs, err := parseRequiredProviderIDs(input.ProviderIDs)
	if err != nil {
		return GroupResponse{}, err
	}
	patterns, err := validatePatterns(input.ModelPatterns)
	if err != nil {
		return GroupResponse{}, err
	}

	var record Group
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.ensureProvidersExist(ctx, tx, providerIDs); err != nil {
			return err
		}

		record = Group{
			Name:        name,
			DisplayName: displayName,
			Description: strings.TrimSpace(input.Description),
		}
		if err := tx.WithContext(ctx).Create(&record).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrNameConflict
			}
			return fmt.Errorf("create group: %w", err)
		}
		if err := s.replaceGroupProviders(ctx, tx, record.ID, providerIDs); err != nil {
			return err
		}
		if err := s.replaceGroupPatterns(ctx, tx, record.ID, patterns); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return GroupResponse{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainGroups)
	return s.GetGroup(ctx, record.ID.String())
}

// UpdateGroup patches an access group and reconciles provider/model assignments.
func (s *Service) UpdateGroup(ctx context.Context, id string, input UpdateGroupInput) (GroupResponse, error) {
	var parsedName string
	var err error
	if input.Name != nil {
		parsedName, err = validateName(*input.Name)
		if err != nil {
			return GroupResponse{}, err
		}
	}
	var parsedDisplayName string
	if input.DisplayName != nil {
		parsedDisplayName, err = validateDisplayName(*input.DisplayName)
		if err != nil {
			return GroupResponse{}, err
		}
	}
	var providerIDs []uuid.UUID
	if input.ProviderIDs != nil {
		providerIDs, err = parseRequiredProviderIDs(*input.ProviderIDs)
		if err != nil {
			return GroupResponse{}, err
		}
	}
	var patterns []string
	if input.ModelPatterns != nil {
		patterns, err = validatePatterns(*input.ModelPatterns)
		if err != nil {
			return GroupResponse{}, err
		}
	}

	var groupID uuid.UUID
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, err := s.getGroup(ctx, tx, id)
		if err != nil {
			return err
		}
		groupID = record.ID
		if input.Name != nil {
			record.Name = parsedName
		}
		if input.DisplayName != nil {
			record.DisplayName = parsedDisplayName
		}
		if input.Description != nil {
			record.Description = strings.TrimSpace(*input.Description)
		}
		if err := tx.WithContext(ctx).Save(&record).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrNameConflict
			}
			return fmt.Errorf("update group: %w", err)
		}
		if input.ProviderIDs != nil {
			if err := s.ensureProvidersExist(ctx, tx, providerIDs); err != nil {
				return err
			}
			if err := s.replaceGroupProviders(ctx, tx, record.ID, providerIDs); err != nil {
				return err
			}
		}
		if input.ModelPatterns != nil {
			if err := s.replaceGroupPatterns(ctx, tx, record.ID, patterns); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return GroupResponse{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainGroups)
	return s.GetGroup(ctx, groupID.String())
}

// DeleteGroup removes an access group and its membership and model/provider joins.
func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	groupID, err := parseGroupID(id)
	if err != nil {
		return ErrGroupNotFound
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("group_id = ?", groupID).Delete(&GroupProvider{}).Error; err != nil {
			return fmt.Errorf("delete group providers: %w", err)
		}
		if err := tx.WithContext(ctx).Where("group_id = ?", groupID).Delete(&GroupModelPattern{}).Error; err != nil {
			return fmt.Errorf("delete group model patterns: %w", err)
		}
		if err := tx.WithContext(ctx).Where("group_id = ?", groupID).Delete(&GroupMember{}).Error; err != nil {
			return fmt.Errorf("delete group members: %w", err)
		}
		result := tx.WithContext(ctx).Delete(&Group{}, "id = ?", groupID)
		if result.Error != nil {
			return fmt.Errorf("delete group: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrGroupNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.notifier.Notify(ctx, configevents.DomainGroups)
	return nil
}

// AddMember assigns a user to an access group.
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

// RemoveMember removes a user from an access group.
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

// ListUserGroups returns full access group details assigned to a user.
func (s *Service) ListUserGroups(ctx context.Context, userID string) ([]GroupResponse, error) {
	userID = strings.TrimSpace(userID)
	if err := s.ensureUserExists(ctx, s.db, userID); err != nil {
		return nil, err
	}

	var records []Group
	if err := s.db.WithContext(ctx).
		Joins("JOIN access_group_members ON access_group_members.group_id = access_groups.id").
		Where("access_group_members.user_id = ?", userID).
		Preload("Providers").
		Preload("ModelPatterns").
		Preload("Members").
		Order("access_groups.name ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list user groups: %w", err)
	}
	out := make([]GroupResponse, len(records))
	for i := range records {
		out[i] = records[i].toResponse()
	}
	return out, nil
}

// ListUserGroupSummaries returns profile-safe access group summaries for a user.
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
		Order("access_groups.name ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list user group summaries: %w", err)
	}
	out := make([]ProfileGroupResponse, len(records))
	for i := range records {
		out[i] = records[i].toProfileResponse()
	}
	return out, nil
}

// ReplaceUserGroups replaces all access group assignments for a user.
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

// getGroup loads one access group with all admin response relations.
func (s *Service) getGroup(ctx context.Context, db *gorm.DB, id string) (Group, error) {
	groupID, err := parseGroupID(id)
	if err != nil {
		return Group{}, ErrGroupNotFound
	}
	var record Group
	if err := db.WithContext(ctx).
		Preload("Providers").
		Preload("ModelPatterns").
		Preload("Members").
		First(&record, "id = ?", groupID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Group{}, ErrGroupNotFound
		}
		return Group{}, fmt.Errorf("get group: %w", err)
	}
	return record, nil
}

// ensureGroupExists verifies that an access group exists.
func (s *Service) ensureGroupExists(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	var count int64
	if err := db.WithContext(ctx).Model(&Group{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return fmt.Errorf("check group: %w", err)
	}
	if count == 0 {
		return ErrGroupNotFound
	}
	return nil
}

// ensureGroupsExist verifies that every requested access group exists.
func (s *Service) ensureGroupsExist(ctx context.Context, db *gorm.DB, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if err := db.WithContext(ctx).Model(&Group{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return fmt.Errorf("check groups: %w", err)
	}
	if count != int64(len(ids)) {
		return ErrGroupNotFound
	}
	return nil
}

// ensureProvidersExist verifies that every requested provider exists.
func (s *Service) ensureProvidersExist(ctx context.Context, db *gorm.DB, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if err := db.WithContext(ctx).Model(&provider.Provider{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return fmt.Errorf("check providers: %w", err)
	}
	if count != int64(len(ids)) {
		return ErrProviderNotFound
	}
	return nil
}

// ensureUserExists verifies that a user exists.
func (s *Service) ensureUserExists(ctx context.Context, db *gorm.DB, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrUserNotFound
	}
	var count int64
	if err := db.WithContext(ctx).Model(&users.User{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return fmt.Errorf("check user: %w", err)
	}
	if count == 0 {
		return ErrUserNotFound
	}
	return nil
}

// replaceGroupProviders replaces provider assignments for an access group.
func (s *Service) replaceGroupProviders(ctx context.Context, tx *gorm.DB, groupID uuid.UUID, providerIDs []uuid.UUID) error {
	if err := tx.WithContext(ctx).Where("group_id = ?", groupID).Delete(&GroupProvider{}).Error; err != nil {
		return fmt.Errorf("delete group providers: %w", err)
	}
	for _, providerID := range providerIDs {
		if err := tx.WithContext(ctx).Create(&GroupProvider{GroupID: groupID, ProviderID: providerID}).Error; err != nil {
			return fmt.Errorf("create group provider: %w", err)
		}
	}
	return nil
}

// replaceGroupPatterns replaces model pattern assignments for an access group.
func (s *Service) replaceGroupPatterns(ctx context.Context, tx *gorm.DB, groupID uuid.UUID, patterns []string) error {
	if err := tx.WithContext(ctx).Where("group_id = ?", groupID).Delete(&GroupModelPattern{}).Error; err != nil {
		return fmt.Errorf("delete group model patterns: %w", err)
	}
	for _, pattern := range patterns {
		if err := tx.WithContext(ctx).Create(&GroupModelPattern{GroupID: groupID, Pattern: pattern}).Error; err != nil {
			return fmt.Errorf("create group model pattern: %w", err)
		}
	}
	return nil
}

// validateName normalizes and validates an access group slug.
func validateName(raw string) (string, error) {
	name := normalizeName(raw)
	if !groupNameRegexp.MatchString(name) {
		return "", ErrInvalidName
	}
	return name, nil
}

// validateDisplayName trims and validates an access group display name.
func validateDisplayName(raw string) (string, error) {
	displayName := strings.TrimSpace(raw)
	if displayName == "" {
		return "", ErrInvalidDisplayName
	}
	return displayName, nil
}

// validatePatterns compiles unique model patterns and applies the all-model default when empty.
func validatePatterns(raw []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, pattern := range raw {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return nil, ErrInvalidRegex
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		out = append(out, pattern)
	}
	if len(out) == 0 {
		return []string{defaultAllModelsPattern}, nil
	}
	return out, nil
}

// parseGroupID parses a trimmed access group UUID.
func parseGroupID(raw string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(raw))
}

// parseGroupIDs parses and de-duplicates access group UUIDs.
func parseGroupIDs(raw []string) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	out := make([]uuid.UUID, 0, len(raw))
	for _, id := range raw {
		parsed, err := parseGroupID(id)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[parsed]; ok {
			continue
		}
		seen[parsed] = struct{}{}
		out = append(out, parsed)
	}
	return out, nil
}

// parseProviderIDs parses and de-duplicates provider UUIDs.
func parseProviderIDs(raw []string) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	out := make([]uuid.UUID, 0, len(raw))
	for _, id := range raw {
		parsed, err := uuid.Parse(strings.TrimSpace(id))
		if err != nil {
			return nil, ErrProviderNotFound
		}
		if _, ok := seen[parsed]; ok {
			continue
		}
		seen[parsed] = struct{}{}
		out = append(out, parsed)
	}
	return out, nil
}

// parseRequiredProviderIDs parses provider UUIDs and rejects empty assignments.
func parseRequiredProviderIDs(raw []string) ([]uuid.UUID, error) {
	out, err := parseProviderIDs(raw)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrProviderRequired
	}
	return out, nil
}

// normalizeListParams applies default access group pagination and sorting values.
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
		params.SortBy = "name"
	}
	if params.SortDir == "" {
		params.SortDir = "asc"
	}
}

// applyGroupSort applies a validated access group order to the query.
func applyGroupSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}
	columns := map[string]string{
		"name":              "access_groups.name",
		"displayName":       "access_groups.display_name",
		"providerCount":     "(SELECT COUNT(*) FROM access_group_providers WHERE access_group_providers.group_id = access_groups.id)",
		"modelPatternCount": "(SELECT COUNT(*) FROM access_group_model_patterns WHERE access_group_model_patterns.group_id = access_groups.id)",
		"memberCount":       "(SELECT COUNT(*) FROM access_group_members WHERE access_group_members.group_id = access_groups.id)",
		"createdAt":         "access_groups.created_at",
		"updatedAt":         "access_groups.updated_at",
	}
	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	return query.Order(column + " " + dir).Order("access_groups.id ASC"), nil
}

// normalizeSortDir converts an access group sort direction into SQL syntax.
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

// isUniqueConstraintError detects database uniqueness violations.
func isUniqueConstraintError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "23505")
}

// toResponse converts an access group model into its admin API shape.
func (g *Group) toResponse() GroupResponse {
	providers := make([]ProviderSummary, 0, len(g.Providers))
	for _, item := range g.Providers {
		providers = append(providers, ProviderSummary{
			ID:          item.ID,
			Name:        item.Name,
			DisplayName: item.DisplayName,
			Type:        item.Type,
			Enabled:     item.Enabled,
		})
	}
	patterns := make([]string, 0, len(g.ModelPatterns))
	for _, item := range g.ModelPatterns {
		patterns = append(patterns, item.Pattern)
	}
	members := make([]MemberSummary, 0, len(g.Members))
	for _, item := range g.Members {
		members = append(members, MemberSummary{
			ID:                item.ID,
			PreferredUsername: item.PreferredUsername,
			Email:             item.Email,
			Name:              item.Name,
			Type:              item.Type,
			Role:              item.Role,
			IsActive:          item.IsActive,
		})
	}
	return GroupResponse{
		ID:                g.ID,
		Name:              g.Name,
		DisplayName:       g.DisplayName,
		Description:       g.Description,
		Providers:         providers,
		ModelPatterns:     patterns,
		Members:           members,
		ProviderCount:     len(providers),
		ModelPatternCount: len(patterns),
		MemberCount:       len(members),
		CreatedAt:         g.CreatedAt,
		UpdatedAt:         g.UpdatedAt,
	}
}

// toProfileResponse converts an access group model into its profile API shape.
func (g *Group) toProfileResponse() ProfileGroupResponse {
	return ProfileGroupResponse{
		ID:          g.ID,
		Name:        g.Name,
		DisplayName: g.DisplayName,
		Description: g.Description,
	}
}
