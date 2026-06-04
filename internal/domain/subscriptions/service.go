package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/configevents"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPlanNotFound      = errors.New("subscription plan not found")
	ErrInvalidPlan       = errors.New("invalid subscription plan")
	ErrDefaultPlanDelete = errors.New("default subscription plan cannot be deleted")
	ErrPlanAssigned      = errors.New("subscription plan is assigned to accounts")
	ErrInvalidAssignment = errors.New("invalid subscription plan assignment")
	ErrInvalidSort       = errors.New("invalid_sort")
)

const (
	maxPlanNameLength        = 120
	maxPlanDescriptionLength = 2000
)

type Service struct {
	db       *gorm.DB
	notifier configevents.Notifier
}

type userPlanRow struct {
	ID                 string
	SubscriptionPlanID *string
}

type planAssignmentCountRow struct {
	SubscriptionPlanID string
	Type               auth.UserType
	Count              int64
}

type planAssignmentCounts struct {
	Users           int64
	ServiceAccounts int64
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, notifier: configevents.NoopNotifier{}}
}

func (s *Service) SetNotifier(notifier configevents.Notifier) {
	if notifier == nil {
		notifier = configevents.NoopNotifier{}
	}
	s.notifier = notifier
}

func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&SubscriptionPlan{}, &SubscriptionQuotaState{})
}

func (s *Service) CreatePlan(ctx context.Context, input PlanInput) (PlanResponse, error) {
	plan, err := normalizedPlan(input)
	if err != nil {
		return PlanResponse{}, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if plan.IsDefault {
			if err := clearDefaultPlan(ctx, tx); err != nil {
				return err
			}
		}
		if err := tx.Create(&plan).Error; err != nil {
			return fmt.Errorf("create subscription plan: %w", err)
		}
		return nil
	})
	if err != nil {
		return PlanResponse{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainSubscriptions)
	return planResponse(plan), nil
}

func (s *Service) ListPlans(ctx context.Context) ([]PlanResponse, error) {
	result, err := s.ListPlansPaged(ctx, PlanListParams{
		Page:     1,
		PageSize: 100,
		SortBy:   "name",
		SortDir:  "asc",
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *Service) ListPlansPaged(ctx context.Context, params PlanListParams) (PlanListResult, error) {
	normalizePlanListParams(&params)
	query := s.db.WithContext(ctx).Model(&SubscriptionPlan{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return PlanListResult{}, fmt.Errorf("count subscription plans: %w", err)
	}

	query, err := applyPlanSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return PlanListResult{}, err
	}

	var records []SubscriptionPlan
	if err := query.
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&records).Error; err != nil {
		return PlanListResult{}, fmt.Errorf("list subscription plans: %w", err)
	}

	items := make([]PlanResponse, 0, len(records))
	assignmentCounts, err := s.planAssignmentCountLookup(ctx, planIDs(records))
	if err != nil {
		return PlanListResult{}, err
	}
	for _, record := range records {
		items = append(items, planResponseWithAssignmentCounts(record, assignmentCounts[record.ID]))
	}
	return PlanListResult{
		Items:    items,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

func (s *Service) GetPlan(ctx context.Context, id string) (PlanResponse, error) {
	plan, err := s.findPlan(ctx, s.db, id)
	if err != nil {
		return PlanResponse{}, err
	}
	return s.planResponseWithCurrentAssignmentCounts(ctx, plan)
}

func (s *Service) UpdatePlan(ctx context.Context, id string, input PlanInput) (PlanResponse, error) {
	next, err := normalizedPlan(input)
	if err != nil {
		return PlanResponse{}, err
	}

	var plan SubscriptionPlan
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		loaded, err := s.findPlan(ctx, tx, id)
		if err != nil {
			return err
		}
		plan = loaded
		if next.IsDefault {
			if err := clearDefaultPlan(ctx, tx); err != nil {
				return err
			}
		}
		plan.Name = next.Name
		plan.Description = next.Description
		plan.Quota5HTokens = next.Quota5HTokens
		plan.Quota7DTokens = next.Quota7DTokens
		plan.IsDefault = next.IsDefault
		if err := tx.Save(&plan).Error; err != nil {
			return fmt.Errorf("update subscription plan: %w", err)
		}
		return nil
	})
	if err != nil {
		return PlanResponse{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainSubscriptions)
	return s.planResponseWithCurrentAssignmentCounts(ctx, plan)
}

func (s *Service) DeletePlan(ctx context.Context, id string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		plan, err := s.findPlan(ctx, tx, id)
		if err != nil {
			return err
		}
		if plan.IsDefault {
			return ErrDefaultPlanDelete
		}
		var assigned int64
		if err := tx.Model(&users.User{}).
			Where("subscription_plan_id = ?", id).
			Count(&assigned).Error; err != nil {
			return fmt.Errorf("count assigned subscription plan accounts: %w", err)
		}
		if assigned > 0 {
			return ErrPlanAssigned
		}
		if err := tx.Delete(&SubscriptionPlan{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete subscription plan: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.notifier.Notify(ctx, configevents.DomainSubscriptions)
	return nil
}

func (s *Service) SetDefaultPlan(ctx context.Context, id string) (PlanResponse, error) {
	var plan SubscriptionPlan
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		loaded, err := s.findPlan(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := clearDefaultPlan(ctx, tx); err != nil {
			return err
		}
		loaded.IsDefault = true
		if err := tx.Save(&loaded).Error; err != nil {
			return fmt.Errorf("set default subscription plan: %w", err)
		}
		plan = loaded
		return nil
	})
	if err != nil {
		return PlanResponse{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainSubscriptions)
	return s.planResponseWithCurrentAssignmentCounts(ctx, plan)
}

func (s *Service) AssignUserPlan(ctx context.Context, userID string, planID *string) (users.AdminUser, error) {
	if err := s.assignPlan(ctx, userID, auth.UserTypeUser, planID); err != nil {
		return users.AdminUser{}, err
	}
	var record users.User
	if err := s.db.WithContext(ctx).First(&record, "id = ? AND type = ?", userID, auth.UserTypeUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return users.AdminUser{}, users.ErrUserNotFound
		}
		return users.AdminUser{}, fmt.Errorf("load assigned user: %w", err)
	}
	items := []users.AdminUser{recordToAdminUser(record)}
	if err := s.DecorateAdminUsers(ctx, items); err != nil {
		return users.AdminUser{}, err
	}
	s.notifier.Notify(ctx, configevents.DomainSubscriptions)
	return items[0], nil
}

func (s *Service) AssignServiceAccountPlan(ctx context.Context, userID string, planID *string) (users.ServiceAccount, error) {
	if err := s.assignPlan(ctx, userID, auth.UserTypeService, planID); err != nil {
		return users.ServiceAccount{}, err
	}
	var record users.User
	if err := s.db.WithContext(ctx).First(&record, "id = ? AND type = ?", userID, auth.UserTypeService).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return users.ServiceAccount{}, users.ErrUserNotFound
		}
		return users.ServiceAccount{}, fmt.Errorf("load assigned service account: %w", err)
	}
	items := []users.ServiceAccount{recordToServiceAccount(record)}
	if err := s.DecorateServiceAccounts(ctx, items); err != nil {
		return users.ServiceAccount{}, err
	}
	s.notifier.Notify(ctx, configevents.DomainSubscriptions)
	return items[0], nil
}

func (s *Service) UserPlanID(ctx context.Context, userID string) (*string, bool, error) {
	var row userPlanRow
	if err := s.db.WithContext(ctx).
		Model(&users.User{}).
		Select("id, subscription_plan_id").
		Where("id = ?", userID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("load subscription assignment: %w", err)
	}
	return cloneStringPtr(row.SubscriptionPlanID), true, nil
}

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	var plans []SubscriptionPlan
	if err := s.db.WithContext(ctx).Find(&plans).Error; err != nil {
		return Snapshot{}, fmt.Errorf("load subscription plans snapshot: %w", err)
	}
	snapshot := Snapshot{
		Plans:     make(map[string]PlanSnapshot, len(plans)),
		CreatedAt: time.Now().UTC(),
	}
	for _, plan := range plans {
		planSnapshot := snapshotFromPlan(plan)
		snapshot.Plans[plan.ID] = planSnapshot
		if plan.IsDefault {
			id := plan.ID
			snapshot.DefaultPlanID = &id
		}
	}
	return snapshot, nil
}

func (s *Service) UpsertQuotaState(ctx context.Context, userID string, status QuotaStatus, syncedAt time.Time) error {
	syncedAt = syncedAt.UTC()
	state := SubscriptionQuotaState{
		UserID:          userID,
		HasSubscription: status.HasSubscription,
		Used5HTokens:    status.Used5HTokens,
		Quota5HTokens:   cloneInt64Ptr(status.Quota5HTokens),
		Reset5HAt:       cloneTimePtr(status.Reset5HAt),
		Used7DTokens:    status.Used7DTokens,
		Quota7DTokens:   cloneInt64Ptr(status.Quota7DTokens),
		Reset7DAt:       cloneTimePtr(status.Reset7DAt),
		SyncedAt:        syncedAt,
	}
	if status.Plan != nil {
		state.PlanID = &status.Plan.ID
		state.PlanName = status.Plan.Name
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"has_subscription": status.HasSubscription,
			"plan_id":          state.PlanID,
			"plan_name":        state.PlanName,
			"used_5h_tokens":   status.Used5HTokens,
			"quota_5h_tokens":  state.Quota5HTokens,
			"reset_5h_at":      state.Reset5HAt,
			"used_7d_tokens":   status.Used7DTokens,
			"quota_7d_tokens":  state.Quota7DTokens,
			"reset_7d_at":      state.Reset7DAt,
			"synced_at":        syncedAt,
			"updated_at":       syncedAt,
		}),
	}).Create(&state).Error
}

func (s *Service) DecorateAdminUsers(ctx context.Context, items []users.AdminUser) error {
	if len(items) == 0 {
		return nil
	}
	plans, defaultPlan, err := s.planLookup(ctx)
	if err != nil {
		return err
	}
	states, err := s.quotaStateLookup(ctx, adminUserIDs(items))
	if err != nil {
		return err
	}
	for i := range items {
		decorateAccount(
			items[i].ID,
			items[i].SubscriptionPlanID,
			plans,
			defaultPlan,
			states,
			&items[i].SubscriptionPlan,
			&items[i].EffectiveSubscriptionPlan,
			&items[i].QuotaState,
		)
	}
	return nil
}

func (s *Service) DecorateServiceAccounts(ctx context.Context, items []users.ServiceAccount) error {
	if len(items) == 0 {
		return nil
	}
	plans, defaultPlan, err := s.planLookup(ctx)
	if err != nil {
		return err
	}
	states, err := s.quotaStateLookup(ctx, serviceAccountIDs(items))
	if err != nil {
		return err
	}
	for i := range items {
		decorateAccount(
			items[i].ID,
			items[i].SubscriptionPlanID,
			plans,
			defaultPlan,
			states,
			&items[i].SubscriptionPlan,
			&items[i].EffectiveSubscriptionPlan,
			&items[i].QuotaState,
		)
	}
	return nil
}

func (s *Service) findPlan(ctx context.Context, tx *gorm.DB, id string) (SubscriptionPlan, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return SubscriptionPlan{}, ErrPlanNotFound
	}
	var plan SubscriptionPlan
	if err := tx.WithContext(ctx).First(&plan, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SubscriptionPlan{}, ErrPlanNotFound
		}
		return SubscriptionPlan{}, fmt.Errorf("load subscription plan: %w", err)
	}
	return plan, nil
}

func (s *Service) assignPlan(ctx context.Context, userID string, userType auth.UserType, planID *string) error {
	normalizedPlanID := normalizeOptionalID(planID)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if normalizedPlanID != nil {
			if _, err := s.findPlan(ctx, tx, *normalizedPlanID); err != nil {
				if errors.Is(err, ErrPlanNotFound) {
					return ErrInvalidAssignment
				}
				return err
			}
		}
		result := tx.Model(&users.User{}).
			Where("id = ? AND type = ?", userID, userType).
			Update("subscription_plan_id", normalizedPlanID)
		if result.Error != nil {
			return fmt.Errorf("assign subscription plan: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return users.ErrUserNotFound
		}
		return nil
	})
}

func normalizedPlan(input PlanInput) (SubscriptionPlan, error) {
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	if name == "" || len([]rune(name)) > maxPlanNameLength {
		return SubscriptionPlan{}, ErrInvalidPlan
	}
	if len([]rune(description)) > maxPlanDescriptionLength {
		return SubscriptionPlan{}, ErrInvalidPlan
	}
	if input.Quota5HTokens != nil && *input.Quota5HTokens <= 0 {
		return SubscriptionPlan{}, ErrInvalidPlan
	}
	if input.Quota7DTokens != nil && *input.Quota7DTokens <= 0 {
		return SubscriptionPlan{}, ErrInvalidPlan
	}
	return SubscriptionPlan{
		Name:          name,
		Description:   description,
		Quota5HTokens: cloneInt64Ptr(input.Quota5HTokens),
		Quota7DTokens: cloneInt64Ptr(input.Quota7DTokens),
		IsDefault:     input.IsDefault,
	}, nil
}

func clearDefaultPlan(ctx context.Context, tx *gorm.DB) error {
	if err := tx.WithContext(ctx).Model(&SubscriptionPlan{}).
		Where("is_default = ?", true).
		Update("is_default", false).Error; err != nil {
		return fmt.Errorf("clear default subscription plan: %w", err)
	}
	return nil
}

func normalizePlanListParams(params *PlanListParams) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	if strings.TrimSpace(params.SortBy) == "" {
		params.SortBy = "name"
	}
	if strings.TrimSpace(params.SortDir) == "" {
		params.SortDir = "asc"
	}
}

func applyPlanSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}
	columns := map[string]string{
		"name":          "name",
		"quota5hTokens": "quota_5h_tokens",
		"quota7dTokens": "quota_7d_tokens",
		"isDefault":     "is_default",
		"createdAt":     "created_at",
		"updatedAt":     "updated_at",
	}
	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	return query.Order(column + " " + dir).Order("id ASC"), nil
}

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

func normalizeOptionalID(id *string) *string {
	if id == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*id)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func planIDs(records []SubscriptionPlan) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}

func planResponseWithAssignmentCounts(plan SubscriptionPlan, counts planAssignmentCounts) PlanResponse {
	response := planResponse(plan)
	response.AssignedUsersCount = counts.Users
	response.AssignedServiceAccountsCount = counts.ServiceAccounts
	response.AssignedAccountsCount = counts.Users + counts.ServiceAccounts
	return response
}

func (s *Service) planResponseWithCurrentAssignmentCounts(ctx context.Context, plan SubscriptionPlan) (PlanResponse, error) {
	counts, err := s.planAssignmentCountLookup(ctx, []string{plan.ID})
	if err != nil {
		return PlanResponse{}, err
	}
	return planResponseWithAssignmentCounts(plan, counts[plan.ID]), nil
}

func (s *Service) planAssignmentCountLookup(ctx context.Context, planIDs []string) (map[string]planAssignmentCounts, error) {
	countsByPlanID := make(map[string]planAssignmentCounts, len(planIDs))
	if len(planIDs) == 0 {
		return countsByPlanID, nil
	}

	var rows []planAssignmentCountRow
	if err := s.db.WithContext(ctx).
		Model(&users.User{}).
		Select("subscription_plan_id, type, COUNT(*) AS count").
		Where("subscription_plan_id IN ?", planIDs).
		Group("subscription_plan_id, type").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count subscription plan assignments: %w", err)
	}

	for _, row := range rows {
		counts := countsByPlanID[row.SubscriptionPlanID]
		switch row.Type {
		case auth.UserTypeUser:
			counts.Users = row.Count
		case auth.UserTypeService:
			counts.ServiceAccounts = row.Count
		}
		countsByPlanID[row.SubscriptionPlanID] = counts
	}
	return countsByPlanID, nil
}

func (s *Service) planLookup(ctx context.Context) (map[string]users.AccountSubscriptionPlan, *users.AccountSubscriptionPlan, error) {
	var plans []SubscriptionPlan
	if err := s.db.WithContext(ctx).Find(&plans).Error; err != nil {
		return nil, nil, fmt.Errorf("load subscription plan lookup: %w", err)
	}
	lookup := make(map[string]users.AccountSubscriptionPlan, len(plans))
	var defaultPlan *users.AccountSubscriptionPlan
	for _, plan := range plans {
		accountPlan := accountPlanFromResponse(planResponse(plan))
		lookup[plan.ID] = accountPlan
		if plan.IsDefault {
			clone := accountPlan
			defaultPlan = &clone
		}
	}
	return lookup, defaultPlan, nil
}

func (s *Service) quotaStateLookup(ctx context.Context, ids []string) (map[string]SubscriptionQuotaState, error) {
	var states []SubscriptionQuotaState
	if err := s.db.WithContext(ctx).Where("user_id IN ?", ids).Find(&states).Error; err != nil {
		return nil, fmt.Errorf("load subscription quota states: %w", err)
	}
	lookup := make(map[string]SubscriptionQuotaState, len(states))
	for _, state := range states {
		lookup[state.UserID] = state
	}
	return lookup, nil
}

func decorateAccount(
	userID string,
	explicitPlanID *string,
	plans map[string]users.AccountSubscriptionPlan,
	defaultPlan *users.AccountSubscriptionPlan,
	states map[string]SubscriptionQuotaState,
	explicitTarget **users.AccountSubscriptionPlan,
	effectiveTarget **users.AccountSubscriptionPlan,
	quotaTarget **users.AccountQuotaState,
) {
	if explicitPlanID != nil {
		if plan, ok := plans[*explicitPlanID]; ok {
			clone := plan
			*explicitTarget = &clone
			*effectiveTarget = &clone
		}
	} else if defaultPlan != nil {
		clone := *defaultPlan
		*effectiveTarget = &clone
	}
	if state, ok := states[userID]; ok {
		*quotaTarget = quotaStateToAccount(state)
	}
}

func quotaStateToAccount(state SubscriptionQuotaState) *users.AccountQuotaState {
	syncedAt := state.SyncedAt
	return &users.AccountQuotaState{
		HasSubscription: state.HasSubscription,
		PlanID:          cloneStringPtr(state.PlanID),
		PlanName:        state.PlanName,
		Used5HTokens:    state.Used5HTokens,
		Quota5HTokens:   cloneInt64Ptr(state.Quota5HTokens),
		Reset5HAt:       cloneTimePtr(state.Reset5HAt),
		Used7DTokens:    state.Used7DTokens,
		Quota7DTokens:   cloneInt64Ptr(state.Quota7DTokens),
		Reset7DAt:       cloneTimePtr(state.Reset7DAt),
		SyncedAt:        &syncedAt,
	}
}

func adminUserIDs(items []users.AdminUser) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func serviceAccountIDs(items []users.ServiceAccount) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	next := *value
	return &next
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	next := value.UTC()
	return &next
}

func remainingTokens(quota *int64, used int64) *int64 {
	if quota == nil {
		return nil
	}
	remaining := *quota - used
	if remaining < 0 {
		remaining = 0
	}
	return &remaining
}

func quotaStatusFromPlan(plan PlanSnapshot, used5h int64, reset5h *time.Time, used7d int64, reset7d *time.Time) QuotaStatus {
	response := responseFromSnapshot(plan)
	return QuotaStatus{
		HasSubscription:   true,
		Plan:              &response,
		Used5HTokens:      used5h,
		Quota5HTokens:     cloneInt64Ptr(plan.Quota5HTokens),
		Remaining5HTokens: remainingTokens(plan.Quota5HTokens, used5h),
		Reset5HAt:         cloneTimePtr(reset5h),
		Used7DTokens:      used7d,
		Quota7DTokens:     cloneInt64Ptr(plan.Quota7DTokens),
		Remaining7DTokens: remainingTokens(plan.Quota7DTokens, used7d),
		Reset7DAt:         cloneTimePtr(reset7d),
	}
}

func recordToAdminUser(record users.User) users.AdminUser {
	return users.AdminUser{
		ID:                 record.ID,
		Sub:                record.ExternalSub,
		PreferredUsername:  record.PreferredUsername,
		Email:              record.Email,
		Name:               record.Name,
		Type:               record.Type,
		Role:               record.Role,
		SubscriptionPlanID: record.SubscriptionPlanID,
		Note:               record.Note,
		IsActive:           record.IsActive,
		ExpiresAt:          record.ExpiresAt,
		LastLoginAt:        record.LastLoginAt,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

func recordToServiceAccount(record users.User) users.ServiceAccount {
	return users.ServiceAccount{
		ID:                      record.ID,
		Identifier:              record.PreferredUsername,
		Name:                    record.Name,
		Role:                    auth.RoleUser,
		SubscriptionPlanID:      record.SubscriptionPlanID,
		Note:                    record.Note,
		IsActive:                record.IsActive,
		FirewallOverrideEnabled: record.FirewallOverrideEnabled,
		CreatedAt:               record.CreatedAt,
		UpdatedAt:               record.UpdatedAt,
	}
}

func (s *Service) logQuotaSync(ctx context.Context, store *RedisStore) {
	count, err := store.SyncQuotaStates(ctx, s)
	if err != nil {
		slog.Error("failed to sync subscription quota states", "error", err)
		return
	}
	if count > 0 {
		slog.Info("synced subscription quota states", "users", count)
	}
}

func (s *Service) StartQuotaStateSync(ctx context.Context, store *RedisStore, interval time.Duration) {
	if store == nil || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.logQuotaSync(context.Background(), store)
			}
		}
	}()
}
