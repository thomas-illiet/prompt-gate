package subscriptions

import (
	"context"
	"fmt"

	"promptgate/backend/internal/domain/users"
)

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
