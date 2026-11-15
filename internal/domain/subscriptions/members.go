package subscriptions

import (
	"context"
	"fmt"
	"strings"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"

	"github.com/google/uuid"
)

type PlanMember struct {
	ID                string        `json:"id"`
	Name              string        `json:"name"`
	PreferredUsername string        `json:"preferredUsername"`
	Email             string        `json:"email"`
	Type              auth.UserType `json:"type"`
	Inherited         bool          `json:"inherited"`
}

type PlanMemberListResult struct {
	Items    []PlanMember `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Total    int64        `json:"total"`
}

func (s *Service) ListPlanMembers(ctx context.Context, id string, page, pageSize int, search string) (PlanMemberListResult, error) {
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return PlanMemberListResult{}, ErrPlanNotFound
	}
	plan, err := s.findPlan(ctx, s.db, id)
	if err != nil {
		return PlanMemberListResult{}, err
	}

	page = max(page, 1)
	if pageSize <= 0 {
		pageSize = 10
	}
	pageSize = min(pageSize, 100)
	result := PlanMemberListResult{Items: []PlanMember{}, Page: page, PageSize: pageSize}

	query := s.db.WithContext(ctx).Model(&users.User{})
	if plan.IsDefault {
		query = query.Where("(subscription_plan_id = ? OR subscription_plan_id IS NULL)", plan.ID)
	} else {
		query = query.Where("subscription_plan_id = ?", plan.ID)
	}
	if search = strings.TrimSpace(search); search != "" {
		escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(search))
		pattern := "%" + escaped + "%"
		query = query.Where(`(LOWER(name) LIKE ? ESCAPE '!' OR LOWER(preferred_username) LIKE ? ESCAPE '!'
			OR LOWER(email) LIKE ? ESCAPE '!' OR LOWER(CAST(id AS TEXT)) LIKE ? ESCAPE '!')`,
			pattern, pattern, pattern, pattern)
	}

	if err := query.Count(&result.Total).Error; err != nil {
		return PlanMemberListResult{}, fmt.Errorf("count subscription plan members: %w", err)
	}
	if result.Total == 0 || int64(page-1) > (result.Total-1)/int64(pageSize) {
		return result, nil
	}
	if err := query.
		Select("id, name, preferred_username, email, type, (subscription_plan_id IS NULL) AS inherited").
		Order("LOWER(name), LOWER(preferred_username), id").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&result.Items).Error; err != nil {
		return PlanMemberListResult{}, fmt.Errorf("list subscription plan members: %w", err)
	}
	return result, nil
}
