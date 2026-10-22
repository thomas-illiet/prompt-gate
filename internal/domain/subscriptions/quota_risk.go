package subscriptions

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
)

const (
	// QuotaRiskWarningPercent is the first usage percentage that needs attention.
	QuotaRiskWarningPercent = 80.0
	// QuotaRiskCriticalPercent is the first usage percentage considered exhausted.
	QuotaRiskCriticalPercent = 100.0

	quotaRiskTopLimit = 5
)

// QuotaRiskStatus describes the actionable severity of an account quota.
type QuotaRiskStatus string

const (
	QuotaRiskStatusWarning  QuotaRiskStatus = "warning"
	QuotaRiskStatusCritical QuotaRiskStatus = "critical"
)

// DashboardQuotaRiskResponse summarizes accounts approaching or exceeding a finite quota.
type DashboardQuotaRiskResponse struct {
	GeneratedAt        time.Time       `json:"generatedAt"`
	ProjectionSyncedAt *time.Time      `json:"projectionSyncedAt"`
	AtRiskCount        int             `json:"atRiskCount"`
	ExhaustedCount     int             `json:"exhaustedCount"`
	Items              []QuotaRiskItem `json:"items"`
}

// QuotaRiskItem describes the most saturated finite quota window for one account.
type QuotaRiskItem struct {
	AccountID       string          `json:"accountId"`
	AccountType     auth.UserType   `json:"accountType"`
	DisplayName     string          `json:"displayName"`
	PlanName        string          `json:"planName"`
	Window          string          `json:"window"`
	UsedTokens      int64           `json:"usedTokens"`
	QuotaTokens     int64           `json:"quotaTokens"`
	RemainingTokens int64           `json:"remainingTokens"`
	Percent         float64         `json:"percent"`
	ResetAt         *time.Time      `json:"resetAt"`
	SyncedAt        time.Time       `json:"syncedAt"`
	Status          QuotaRiskStatus `json:"status"`
}

type dashboardQuotaRiskRow struct {
	UserID            string        `gorm:"column:user_id"`
	AccountType       auth.UserType `gorm:"column:account_type"`
	Name              string        `gorm:"column:name"`
	PreferredUsername string        `gorm:"column:preferred_username"`
	Email             string        `gorm:"column:email"`
	HasSubscription   bool          `gorm:"column:has_subscription"`
	PlanName          string        `gorm:"column:plan_name"`
	Used5HTokens      int64         `gorm:"column:used_5h_tokens"`
	Quota5HTokens     *int64        `gorm:"column:quota_5h_tokens"`
	Reset5HAt         *time.Time    `gorm:"column:reset_5h_at"`
	Used7DTokens      int64         `gorm:"column:used_7d_tokens"`
	Quota7DTokens     *int64        `gorm:"column:quota_7d_tokens"`
	Reset7DAt         *time.Time    `gorm:"column:reset_7d_at"`
	SyncedAt          time.Time     `gorm:"column:synced_at"`
}

// DashboardQuotaRisk reads the persisted quota projection without querying Redis.
// Unlimited windows are ignored and each account is classified by its most saturated
// finite 5-hour or 7-day window.
func (s *Service) DashboardQuotaRisk(ctx context.Context, now time.Time) (DashboardQuotaRiskResponse, error) {
	response := DashboardQuotaRiskResponse{
		GeneratedAt: now.UTC(),
		Items:       make([]QuotaRiskItem, 0, quotaRiskTopLimit),
	}

	var rows []dashboardQuotaRiskRow
	if err := s.db.WithContext(ctx).
		Table("subscription_quota_states AS quota_states").
		Select(`
			quota_states.user_id,
			accounts.type AS account_type,
			accounts.name,
			accounts.preferred_username,
			accounts.email,
			quota_states.has_subscription,
			quota_states.plan_name,
			quota_states.used_5h_tokens,
			quota_states.quota_5h_tokens,
			quota_states.reset_5h_at,
			quota_states.used_7d_tokens,
			quota_states.quota_7d_tokens,
			quota_states.reset_7d_at,
			quota_states.synced_at
		`).
		Joins("JOIN users AS accounts ON accounts.id = quota_states.user_id").
		Where("accounts.type IN ?", []auth.UserType{auth.UserTypeUser, auth.UserTypeService}).
		Scan(&rows).Error; err != nil {
		return DashboardQuotaRiskResponse{}, fmt.Errorf("load dashboard quota risk projection: %w", err)
	}

	items := make([]QuotaRiskItem, 0, len(rows))
	for _, row := range rows {
		if response.ProjectionSyncedAt == nil || row.SyncedAt.After(*response.ProjectionSyncedAt) {
			syncedAt := row.SyncedAt.UTC()
			response.ProjectionSyncedAt = &syncedAt
		}

		if !row.HasSubscription {
			continue
		}
		item, ok := quotaRiskItemFromRow(row)
		if !ok || item.Percent < QuotaRiskWarningPercent {
			continue
		}
		response.AtRiskCount++
		if item.Status == QuotaRiskStatusCritical {
			response.ExhaustedCount++
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.Status != right.Status {
			return left.Status == QuotaRiskStatusCritical
		}
		if left.Percent != right.Percent {
			return left.Percent > right.Percent
		}
		if left.DisplayName != right.DisplayName {
			return left.DisplayName < right.DisplayName
		}
		return left.AccountID < right.AccountID
	})
	if len(items) > quotaRiskTopLimit {
		items = items[:quotaRiskTopLimit]
	}
	response.Items = items
	return response, nil
}

func quotaRiskItemFromRow(row dashboardQuotaRiskRow) (QuotaRiskItem, bool) {
	windows := []struct {
		name    string
		used    int64
		quota   *int64
		resetAt *time.Time
	}{
		{name: Window5H, used: row.Used5HTokens, quota: row.Quota5HTokens, resetAt: row.Reset5HAt},
		{name: Window7D, used: row.Used7DTokens, quota: row.Quota7DTokens, resetAt: row.Reset7DAt},
	}

	var selected *QuotaRiskItem
	for _, window := range windows {
		if window.quota == nil || *window.quota <= 0 {
			continue
		}
		percent := float64(window.used) * 100 / float64(*window.quota)
		remaining := *window.quota - window.used
		if remaining < 0 {
			remaining = 0
		}
		status := QuotaRiskStatusWarning
		if percent >= QuotaRiskCriticalPercent {
			status = QuotaRiskStatusCritical
		}
		candidate := QuotaRiskItem{
			AccountID:       row.UserID,
			AccountType:     row.AccountType,
			DisplayName:     quotaRiskDisplayName(row),
			PlanName:        row.PlanName,
			Window:          window.name,
			UsedTokens:      window.used,
			QuotaTokens:     *window.quota,
			RemainingTokens: remaining,
			Percent:         percent,
			ResetAt:         cloneTimePtr(window.resetAt),
			SyncedAt:        row.SyncedAt.UTC(),
			Status:          status,
		}
		if selected == nil || candidate.Percent > selected.Percent {
			selected = &candidate
		}
	}
	if selected == nil {
		return QuotaRiskItem{}, false
	}
	return *selected, true
}

func quotaRiskDisplayName(row dashboardQuotaRiskRow) string {
	for _, candidate := range []string{row.Name, row.PreferredUsername, row.Email, row.UserID} {
		if value := strings.TrimSpace(candidate); value != "" {
			return value
		}
	}
	return row.UserID
}
