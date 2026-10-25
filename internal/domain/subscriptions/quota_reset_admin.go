package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"

	"gorm.io/gorm"
)

var (
	ErrInvalidQuotaStateView        = errors.New("invalid quota state view")
	ErrInvalidQuotaAccountType      = errors.New("invalid quota account type")
	ErrInvalidQuotaResetWindow      = errors.New("invalid quota reset window")
	ErrInvalidQuotaResetEventType   = errors.New("invalid quota reset event type")
	ErrInvalidQuotaResetEventStatus = errors.New("invalid quota reset event status")
)

const (
	AdminQuotaStateViewAtRisk = "at-risk"
	AdminQuotaStateViewAll    = "all"

	AdminQuotaResetWindow7D  = "7d"
	AdminQuotaResetWindow30D = "30d"
	AdminQuotaResetWindowAll = "all"

	adminQuotaResetDefaultPageSize = 20
	adminQuotaResetMaxPageSize     = 100
)

// AdminQuotaStateStatus summarizes the most saturated finite quota window.
type AdminQuotaStateStatus string

const (
	AdminQuotaStateUnassigned AdminQuotaStateStatus = "unassigned"
	AdminQuotaStateUnlimited  AdminQuotaStateStatus = "unlimited"
	AdminQuotaStateHealthy    AdminQuotaStateStatus = "healthy"
	AdminQuotaStateWarning    AdminQuotaStateStatus = "warning"
	AdminQuotaStateCritical   AdminQuotaStateStatus = "critical"
)

// AdminQuotaStateParams controls the operational quota projection listing.
type AdminQuotaStateParams struct {
	View        string
	AccountType auth.UserType
	Search      string
	Page        int
	PageSize    int
	SortBy      string
	SortDir     string
}

// AdminQuotaStateWindow is the admin-only projection of one quota window.
type AdminQuotaStateWindow struct {
	Window          string     `json:"window"`
	UsedTokens      int64      `json:"usedTokens"`
	QuotaTokens     *int64     `json:"quotaTokens"`
	RemainingTokens *int64     `json:"remainingTokens"`
	Percent         *float64   `json:"percent"`
	ResetAt         *time.Time `json:"resetAt"`
}

// AdminQuotaStateItem is the admin-only quota state for one projected account.
type AdminQuotaStateItem struct {
	AccountID              string                  `json:"accountId"`
	AccountType            auth.UserType           `json:"accountType"`
	DisplayName            string                  `json:"displayName"`
	HasSubscription        bool                    `json:"hasSubscription"`
	PlanName               string                  `json:"planName"`
	Status                 AdminQuotaStateStatus   `json:"status"`
	MaximumPercent         *float64                `json:"maximumPercent"`
	MostSaturatedWindow    *string                 `json:"mostSaturatedWindow"`
	NextResetAt            *time.Time              `json:"nextResetAt"`
	Windows                []AdminQuotaStateWindow `json:"windows"`
	SyncedAt               time.Time               `json:"syncedAt"`
	searchableDisplayValue string
}

// AdminQuotaStateSummary contains unfiltered counters for every active SQL projection.
type AdminQuotaStateSummary struct {
	ProjectedAccounts int64 `json:"projectedAccounts"`
	AtRiskAccounts    int64 `json:"atRiskAccounts"`
	ExhaustedAccounts int64 `json:"exhaustedAccounts"`
}

// AdminQuotaStateResult is the paginated quota-state response.
type AdminQuotaStateResult struct {
	GeneratedAt        time.Time              `json:"generatedAt"`
	ProjectionSyncedAt *time.Time             `json:"projectionSyncedAt"`
	Summary            AdminQuotaStateSummary `json:"summary"`
	Items              []AdminQuotaStateItem  `json:"items"`
	Page               int                    `json:"page"`
	PageSize           int                    `json:"pageSize"`
	Total              int64                  `json:"total"`
}

// AdminQuotaResetEventParams controls the admin reset-ledger listing.
type AdminQuotaResetEventParams struct {
	Window   string
	Type     QuotaResetEventType
	Status   QuotaResetEventStatus
	Search   string
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}

// AdminQuotaResetEventTarget identifies the account affected by a ledger event.
type AdminQuotaResetEventTarget struct {
	AccountID   string        `json:"accountId"`
	AccountType auth.UserType `json:"accountType"`
	DisplayName string        `json:"displayName"`
}

// AdminQuotaResetEventActor identifies the trusted origin of a ledger event.
type AdminQuotaResetEventActor struct {
	Type        QuotaResetActorType `json:"type"`
	UserID      *string             `json:"userId"`
	DisplayName *string             `json:"displayName"`
}

// AdminQuotaResetEventItem is an operational, admin-only ledger projection.
// It deliberately excludes the internal idempotency key.
type AdminQuotaResetEventItem struct {
	ID                     string                     `json:"id"`
	Target                 AdminQuotaResetEventTarget `json:"target"`
	Type                   QuotaResetEventType        `json:"type"`
	Status                 QuotaResetEventStatus      `json:"status"`
	Delta                  int                        `json:"delta"`
	BalanceAfter           int                        `json:"balanceAfter"`
	Actor                  AdminQuotaResetEventActor  `json:"actor"`
	GrantMonth             *string                    `json:"grantMonth"`
	ResetCutoffAt          *time.Time                 `json:"resetCutoffAt"`
	ReconcileAttempts      int64                      `json:"reconcileAttempts"`
	LastReconcileAttemptAt *time.Time                 `json:"lastReconcileAttemptAt"`
	AppliedAt              *time.Time                 `json:"appliedAt"`
	CreatedAt              time.Time                  `json:"createdAt"`
}

// AdminQuotaResetEventSummary combines period counters with the global pending backlog.
type AdminQuotaResetEventSummary struct {
	MonthlyGrants         int64 `json:"monthlyGrants"`
	AdminGrants           int64 `json:"adminGrants"`
	AppliedResetUses      int64 `json:"appliedResetUses"`
	PendingResetUsesTotal int64 `json:"pendingResetUsesTotal"`
}

// AdminQuotaResetEventResult is the paginated reset-ledger response.
type AdminQuotaResetEventResult struct {
	Window   string                      `json:"window"`
	StartsAt *time.Time                  `json:"startsAt"`
	EndsAt   time.Time                   `json:"endsAt"`
	Summary  AdminQuotaResetEventSummary `json:"summary"`
	Items    []AdminQuotaResetEventItem  `json:"items"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"pageSize"`
	Total    int64                       `json:"total"`
}

// AdminQuotaStates lists active SQL quota projections. It never reaches Redis.
func (s *Service) AdminQuotaStates(ctx context.Context, params AdminQuotaStateParams, now time.Time) (AdminQuotaStateResult, error) {
	params, err := normalizeAdminQuotaStateParams(params)
	if err != nil {
		return AdminQuotaStateResult{}, err
	}
	now = now.UTC()
	result := AdminQuotaStateResult{
		GeneratedAt: now,
		Items:       make([]AdminQuotaStateItem, 0),
		Page:        params.Page,
		PageSize:    params.PageSize,
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
		return AdminQuotaStateResult{}, fmt.Errorf("load admin quota states: %w", err)
	}

	items := make([]AdminQuotaStateItem, 0, len(rows))
	for _, row := range rows {
		item := adminQuotaStateItemFromRow(row)
		result.Summary.ProjectedAccounts++
		if item.Status == AdminQuotaStateWarning || item.Status == AdminQuotaStateCritical {
			result.Summary.AtRiskAccounts++
		}
		if item.Status == AdminQuotaStateCritical {
			result.Summary.ExhaustedAccounts++
		}
		if result.ProjectionSyncedAt == nil || item.SyncedAt.After(*result.ProjectionSyncedAt) {
			syncedAt := item.SyncedAt
			result.ProjectionSyncedAt = &syncedAt
		}

		if params.View == AdminQuotaStateViewAtRisk && item.Status != AdminQuotaStateWarning && item.Status != AdminQuotaStateCritical {
			continue
		}
		if params.AccountType != "" && item.AccountType != params.AccountType {
			continue
		}
		if params.Search != "" && !strings.Contains(item.searchableDisplayValue, params.Search) {
			continue
		}
		items = append(items, item)
	}

	sortAdminQuotaStateItems(items, params.SortBy, params.SortDir)
	result.Total = int64(len(items))
	start, end := adminPageBounds(params.Page, params.PageSize, len(items))
	result.Items = items[start:end]
	return result, nil
}

// AdminQuotaResetEvents lists the durable reset ledger for operations staff.
func (s *Service) AdminQuotaResetEvents(ctx context.Context, params AdminQuotaResetEventParams, now time.Time) (AdminQuotaResetEventResult, error) {
	params, err := normalizeAdminQuotaResetEventParams(params)
	if err != nil {
		return AdminQuotaResetEventResult{}, err
	}
	now = now.UTC()
	startsAt := adminQuotaResetStartsAt(params.Window, now)
	result := AdminQuotaResetEventResult{
		Window:   params.Window,
		StartsAt: cloneTimePtr(startsAt),
		EndsAt:   now,
		Items:    make([]AdminQuotaResetEventItem, 0),
		Page:     params.Page,
		PageSize: params.PageSize,
	}

	var periodSummary struct {
		MonthlyGrants    int64 `gorm:"column:monthly_grants"`
		AdminGrants      int64 `gorm:"column:admin_grants"`
		AppliedResetUses int64 `gorm:"column:applied_reset_uses"`
	}
	if err := adminQuotaResetPeriodQuery(s.db.WithContext(ctx), startsAt, now).
		Select(`
			COALESCE(SUM(CASE WHEN type = ? AND status = ? THEN 1 ELSE 0 END), 0) AS monthly_grants,
			COALESCE(SUM(CASE WHEN type = ? AND status = ? THEN 1 ELSE 0 END), 0) AS admin_grants,
			COALESCE(SUM(CASE WHEN type = ? AND status = ? THEN 1 ELSE 0 END), 0) AS applied_reset_uses
		`,
			QuotaResetEventMonthlyGrant, QuotaResetEventApplied,
			QuotaResetEventAdminGrant, QuotaResetEventApplied,
			QuotaResetEventUsed, QuotaResetEventApplied,
		).
		Scan(&periodSummary).Error; err != nil {
		return AdminQuotaResetEventResult{}, fmt.Errorf("summarize admin quota reset events: %w", err)
	}
	result.Summary.MonthlyGrants = periodSummary.MonthlyGrants
	result.Summary.AdminGrants = periodSummary.AdminGrants
	result.Summary.AppliedResetUses = periodSummary.AppliedResetUses

	if err := s.db.WithContext(ctx).
		Model(&QuotaResetEvent{}).
		Where("type = ? AND status = ?", QuotaResetEventUsed, QuotaResetEventPending).
		Count(&result.Summary.PendingResetUsesTotal).Error; err != nil {
		return AdminQuotaResetEventResult{}, fmt.Errorf("count pending quota reset events: %w", err)
	}

	filteredQuery := func() *gorm.DB {
		query := adminQuotaResetPeriodQuery(s.db.WithContext(ctx), startsAt, now).
			Joins("JOIN users AS target_accounts ON target_accounts.id = quota_reset_events.user_id")
		if params.Type != "" {
			query = query.Where("quota_reset_events.type = ?", params.Type)
		}
		if params.Status != "" {
			query = query.Where("quota_reset_events.status = ?", params.Status)
		}
		if params.Search != "" {
			query = query.Where(`
				LOWER(
					COALESCE(target_accounts.name, '') || ' ' ||
					COALESCE(target_accounts.preferred_username, '') || ' ' ||
					COALESCE(target_accounts.email, '') || ' ' ||
					quota_reset_events.user_id
				) LIKE ? ESCAPE '!'
			`, "%"+escapeAdminQuotaResetSearch(params.Search)+"%")
		}
		return query
	}
	if err := filteredQuery().Count(&result.Total).Error; err != nil {
		return AdminQuotaResetEventResult{}, fmt.Errorf("count admin quota reset events: %w", err)
	}

	pageIndex := int64(params.Page - 1)
	if result.Total == 0 || pageIndex > (result.Total-1)/int64(params.PageSize) {
		return result, nil
	}
	offset := int(pageIndex * int64(params.PageSize))
	query := filteredQuery().
		Select("quota_reset_events.*").
		Preload("User").
		Preload("ActorUser")
	for _, order := range adminQuotaResetEventOrder(params.SortBy, params.SortDir) {
		query = query.Order(order)
	}
	var events []QuotaResetEvent
	if err := query.
		Limit(params.PageSize).
		Offset(offset).
		Find(&events).Error; err != nil {
		return AdminQuotaResetEventResult{}, fmt.Errorf("load admin quota reset events: %w", err)
	}
	result.Items = make([]AdminQuotaResetEventItem, 0, len(events))
	for _, event := range events {
		result.Items = append(result.Items, adminQuotaResetEventItem(event))
	}
	return result, nil
}

func adminQuotaResetPeriodQuery(db *gorm.DB, startsAt *time.Time, endsAt time.Time) *gorm.DB {
	query := db.Model(&QuotaResetEvent{}).Where("quota_reset_events.created_at <= ?", endsAt)
	if startsAt != nil {
		query = query.Where("quota_reset_events.created_at >= ?", *startsAt)
	}
	return query
}

func escapeAdminQuotaResetSearch(search string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(search)
}

func adminQuotaResetEventOrder(sortBy, sortDir string) []string {
	direction := "DESC"
	if sortDir == "asc" {
		direction = "ASC"
	}
	var orders []string
	switch sortBy {
	case "appliedAt":
		orders = append(orders,
			"CASE WHEN quota_reset_events.applied_at IS NULL THEN 1 ELSE 0 END ASC",
			"quota_reset_events.applied_at "+direction,
		)
	case "user":
		orders = append(orders, `LOWER(COALESCE(
			NULLIF(TRIM(target_accounts.name), ''),
			NULLIF(TRIM(target_accounts.preferred_username), ''),
			NULLIF(TRIM(target_accounts.email), ''),
			quota_reset_events.user_id
		)) `+direction)
	case "type":
		orders = append(orders, "quota_reset_events.type "+direction)
	case "status":
		orders = append(orders, "quota_reset_events.status "+direction)
	default:
		orders = append(orders, "quota_reset_events.created_at "+direction)
	}
	return append(orders, "quota_reset_events.id ASC")
}

func normalizeAdminQuotaStateParams(params AdminQuotaStateParams) (AdminQuotaStateParams, error) {
	params.View = strings.TrimSpace(params.View)
	if params.View == "" {
		params.View = AdminQuotaStateViewAtRisk
	}
	if params.View != AdminQuotaStateViewAtRisk && params.View != AdminQuotaStateViewAll {
		return AdminQuotaStateParams{}, ErrInvalidQuotaStateView
	}
	params.AccountType = auth.UserType(strings.TrimSpace(string(params.AccountType)))
	if params.AccountType != "" && params.AccountType != auth.UserTypeUser && params.AccountType != auth.UserTypeService {
		return AdminQuotaStateParams{}, ErrInvalidQuotaAccountType
	}
	params.Search = strings.ToLower(strings.TrimSpace(params.Search))
	params.SortBy = strings.TrimSpace(params.SortBy)
	if params.SortBy == "" {
		params.SortBy = "saturation"
	}
	switch params.SortBy {
	case "saturation", "displayName", "resetAt", "syncedAt":
	default:
		return AdminQuotaStateParams{}, ErrInvalidSort
	}
	params.SortDir = strings.ToLower(strings.TrimSpace(params.SortDir))
	if params.SortDir == "" {
		params.SortDir = "desc"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		return AdminQuotaStateParams{}, ErrInvalidSort
	}
	params.Page, params.PageSize = normalizeAdminQuotaResetPage(params.Page, params.PageSize)
	return params, nil
}

func normalizeAdminQuotaResetEventParams(params AdminQuotaResetEventParams) (AdminQuotaResetEventParams, error) {
	params.Window = strings.ToLower(strings.TrimSpace(params.Window))
	if params.Window == "" {
		params.Window = AdminQuotaResetWindow30D
	}
	switch params.Window {
	case AdminQuotaResetWindow7D, AdminQuotaResetWindow30D, AdminQuotaResetWindowAll:
	default:
		return AdminQuotaResetEventParams{}, ErrInvalidQuotaResetWindow
	}
	params.Type = QuotaResetEventType(strings.TrimSpace(string(params.Type)))
	switch params.Type {
	case "", QuotaResetEventMonthlyGrant, QuotaResetEventAdminGrant, QuotaResetEventUsed:
	default:
		return AdminQuotaResetEventParams{}, ErrInvalidQuotaResetEventType
	}
	params.Status = QuotaResetEventStatus(strings.TrimSpace(string(params.Status)))
	switch params.Status {
	case "", QuotaResetEventPending, QuotaResetEventApplied:
	default:
		return AdminQuotaResetEventParams{}, ErrInvalidQuotaResetEventStatus
	}
	params.Search = strings.ToLower(strings.TrimSpace(params.Search))
	params.SortBy = strings.TrimSpace(params.SortBy)
	if params.SortBy == "" {
		params.SortBy = "createdAt"
	}
	switch params.SortBy {
	case "createdAt", "appliedAt", "user", "type", "status":
	default:
		return AdminQuotaResetEventParams{}, ErrInvalidSort
	}
	params.SortDir = strings.ToLower(strings.TrimSpace(params.SortDir))
	if params.SortDir == "" {
		params.SortDir = "desc"
	}
	if params.SortDir != "asc" && params.SortDir != "desc" {
		return AdminQuotaResetEventParams{}, ErrInvalidSort
	}
	params.Page, params.PageSize = normalizeAdminQuotaResetPage(params.Page, params.PageSize)
	return params, nil
}

func normalizeAdminQuotaResetPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = adminQuotaResetDefaultPageSize
	}
	if pageSize > adminQuotaResetMaxPageSize {
		pageSize = adminQuotaResetMaxPageSize
	}
	return page, pageSize
}

func adminPageBounds(page, pageSize, length int) (int, int) {
	if length == 0 {
		return 0, 0
	}
	maxInt := int(^uint(0) >> 1)
	if page > 1 && page-1 > maxInt/pageSize {
		return length, length
	}
	start := (page - 1) * pageSize
	if start >= length {
		return length, length
	}
	end := start + pageSize
	if end > length {
		end = length
	}
	return start, end
}

func adminQuotaStateItemFromRow(row dashboardQuotaRiskRow) AdminQuotaStateItem {
	displayName := quotaRiskDisplayName(row)
	item := AdminQuotaStateItem{
		AccountID:       row.UserID,
		AccountType:     row.AccountType,
		DisplayName:     displayName,
		HasSubscription: row.HasSubscription,
		PlanName:        row.PlanName,
		Status:          AdminQuotaStateUnassigned,
		Windows:         make([]AdminQuotaStateWindow, 0, 2),
		SyncedAt:        row.SyncedAt.UTC(),
		searchableDisplayValue: strings.ToLower(strings.Join([]string{
			row.UserID, displayName, row.Name, row.PreferredUsername, row.Email,
		}, " ")),
	}

	windows := []struct {
		name    string
		used    int64
		quota   *int64
		resetAt *time.Time
	}{
		{name: Window5H, used: row.Used5HTokens, quota: row.Quota5HTokens, resetAt: row.Reset5HAt},
		{name: Window7D, used: row.Used7DTokens, quota: row.Quota7DTokens, resetAt: row.Reset7DAt},
	}
	for _, source := range windows {
		window := AdminQuotaStateWindow{
			Window:     source.name,
			UsedTokens: source.used,
		}
		if source.quota != nil && *source.quota > 0 {
			quota := *source.quota
			remaining := quota - source.used
			if remaining < 0 {
				remaining = 0
			}
			percent := float64(source.used) * 100 / float64(quota)
			window.QuotaTokens = &quota
			window.RemainingTokens = &remaining
			window.Percent = &percent
			window.ResetAt = cloneTimePtr(source.resetAt)
			if item.MaximumPercent == nil || percent > *item.MaximumPercent {
				item.MaximumPercent = &percent
				name := source.name
				item.MostSaturatedWindow = &name
			}
			if source.resetAt != nil && (item.NextResetAt == nil || source.resetAt.Before(*item.NextResetAt)) {
				item.NextResetAt = cloneTimePtr(source.resetAt)
			}
		}
		item.Windows = append(item.Windows, window)
	}

	if !item.HasSubscription {
		for index := range item.Windows {
			item.Windows[index].QuotaTokens = nil
			item.Windows[index].RemainingTokens = nil
			item.Windows[index].Percent = nil
			item.Windows[index].ResetAt = nil
		}
		item.MaximumPercent = nil
		item.MostSaturatedWindow = nil
		item.NextResetAt = nil
		return item
	}
	if item.MaximumPercent == nil {
		item.Status = AdminQuotaStateUnlimited
		return item
	}
	switch {
	case *item.MaximumPercent >= QuotaRiskCriticalPercent:
		item.Status = AdminQuotaStateCritical
	case *item.MaximumPercent >= QuotaRiskWarningPercent:
		item.Status = AdminQuotaStateWarning
	default:
		item.Status = AdminQuotaStateHealthy
	}
	return item
}

func adminQuotaResetStartsAt(window string, now time.Time) *time.Time {
	if window == AdminQuotaResetWindowAll {
		return nil
	}
	days := 30
	if window == AdminQuotaResetWindow7D {
		days = 7
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, -(days - 1))
	return &start
}

func adminQuotaResetEventItem(event QuotaResetEvent) AdminQuotaResetEventItem {
	target := AdminQuotaResetEventTarget{
		AccountID:   event.UserID,
		AccountType: auth.UserTypeUser,
		DisplayName: event.UserID,
	}
	if event.User != nil {
		target.AccountType = event.User.Type
		target.DisplayName = adminAccountDisplayName(*event.User)
	}

	actor := AdminQuotaResetEventActor{Type: event.ActorType}
	if event.ActorUserID != nil {
		actor.UserID = cloneStringPtr(event.ActorUserID)
	}
	if event.ActorUser != nil {
		displayName := adminAccountDisplayName(*event.ActorUser)
		actor.DisplayName = &displayName
	}

	var grantMonth *string
	if event.GrantMonth != nil {
		value := event.GrantMonth.UTC().Format("2006-01")
		grantMonth = &value
	}
	return AdminQuotaResetEventItem{
		ID:                     event.ID,
		Target:                 target,
		Type:                   event.Type,
		Status:                 event.Status,
		Delta:                  event.Delta,
		BalanceAfter:           event.BalanceAfter,
		Actor:                  actor,
		GrantMonth:             grantMonth,
		ResetCutoffAt:          cloneTimePtr(event.ResetCutoffAt),
		ReconcileAttempts:      event.ReconcileAttempts,
		LastReconcileAttemptAt: cloneTimePtr(event.LastReconcileAttemptAt),
		AppliedAt:              cloneTimePtr(event.AppliedAt),
		CreatedAt:              event.CreatedAt.UTC(),
	}
}

func adminAccountDisplayName(account users.User) string {
	for _, candidate := range []string{account.Name, account.PreferredUsername, account.Email, account.ID} {
		if value := strings.TrimSpace(candidate); value != "" {
			return value
		}
	}
	return account.ID
}

func sortAdminQuotaStateItems(items []AdminQuotaStateItem, sortBy, sortDir string) {
	descending := sortDir == "desc"
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if sortBy == "saturation" && (left.MaximumPercent == nil || right.MaximumPercent == nil) && left.MaximumPercent != right.MaximumPercent {
			return left.MaximumPercent != nil
		}
		if sortBy == "resetAt" && (left.NextResetAt == nil || right.NextResetAt == nil) && left.NextResetAt != right.NextResetAt {
			return left.NextResetAt != nil
		}
		comparison := 0
		switch sortBy {
		case "displayName":
			comparison = strings.Compare(strings.ToLower(left.DisplayName), strings.ToLower(right.DisplayName))
		case "resetAt":
			comparison = compareNullableTime(left.NextResetAt, right.NextResetAt)
		case "syncedAt":
			comparison = left.SyncedAt.Compare(right.SyncedAt)
		default:
			comparison = compareNullableFloat(left.MaximumPercent, right.MaximumPercent)
		}
		if comparison == 0 {
			return left.AccountID < right.AccountID
		}
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
}

func compareNullableFloat(left, right *float64) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return -1
	case right == nil:
		return 1
	case *left < *right:
		return -1
	case *left > *right:
		return 1
	default:
		return 0
	}
}

func compareNullableTime(left, right *time.Time) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return -1
	case right == nil:
		return 1
	default:
		return left.Compare(*right)
	}
}
