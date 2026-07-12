package proxy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ListPrompts returns paginated prompt history enriched with token totals.
func (s *Service) ListPrompts(ctx context.Context, userID string, params PromptListParams) (PromptListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		return PromptListResult{}, ErrInvalidPagination
	}
	if params.SortBy == "" {
		params.SortBy = "createdAt"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}

	query := s.db.WithContext(ctx).
		Table("user_prompts").
		Joins("JOIN interceptions ON interceptions.id = user_prompts.interception_id").
		Where("interceptions.initiator_id = ?", userID)
	if promptSortNeedsTokenTotals(params.SortBy) {
		query = query.Joins(promptTokenTotalsJoin())
	}
	if search := strings.TrimSpace(strings.ToLower(params.Search)); search != "" {
		query = query.Where("LOWER(user_prompts.prompt) LIKE ?", "%"+search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return PromptListResult{}, fmt.Errorf("count prompt history: %w", err)
	}

	var rows []promptRow
	var err error
	query, err = applyPromptSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return PromptListResult{}, err
	}
	if err := query.
		Select(`user_prompts.id,
			user_prompts.interception_id,
			user_prompts.provider_response_id,
			interceptions.provider,
			interceptions.provider_type,
			interceptions.model,
			user_prompts.prompt,
			interceptions.started_at,
			interceptions.ended_at,
			user_prompts.created_at`).
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Scan(&rows).Error; err != nil {
		return PromptListResult{}, fmt.Errorf("list prompt history: %w", err)
	}

	items := promptRowsToItems(rows)
	if err := s.attachPromptTokenTotals(ctx, items); err != nil {
		return PromptListResult{}, err
	}

	return PromptListResult{
		Items:    items,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// ListAdminPrompts returns paginated prompt history across all users.
func (s *Service) ListAdminPrompts(ctx context.Context, params AdminPromptListParams) (AdminPromptListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		return AdminPromptListResult{}, ErrInvalidPagination
	}
	if params.SortBy == "" {
		params.SortBy = "createdAt"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}

	query := s.db.WithContext(ctx).
		Table("user_prompts").
		Joins("JOIN interceptions ON interceptions.id = user_prompts.interception_id").
		Joins("JOIN users ON users.id = interceptions.initiator_id")
	if promptSortNeedsTokenTotals(params.SortBy) {
		query = query.Joins(promptTokenTotalsJoin())
	}
	if search := strings.TrimSpace(strings.ToLower(params.Search)); search != "" {
		query = query.Where("LOWER(user_prompts.prompt) LIKE ?", "%"+search+"%")
	}
	if userID := strings.TrimSpace(params.UserID); userID != "" {
		query = query.Where("interceptions.initiator_id = ?", userID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return AdminPromptListResult{}, fmt.Errorf("count admin prompt history: %w", err)
	}

	var rows []adminPromptRow
	var err error
	query, err = applyAdminPromptSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return AdminPromptListResult{}, err
	}
	if err := query.
		Select(`user_prompts.id,
			user_prompts.interception_id,
			user_prompts.provider_response_id,
			interceptions.provider,
			interceptions.provider_type,
			interceptions.model,
			user_prompts.prompt,
			interceptions.initiator_id AS user_id,
			users.name AS user_name,
			users.email AS user_email,
			users.preferred_username AS user_preferred_username,
			interceptions.client_ip,
			interceptions.started_at,
			interceptions.ended_at,
			user_prompts.created_at`).
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Scan(&rows).Error; err != nil {
		return AdminPromptListResult{}, fmt.Errorf("list admin prompt history: %w", err)
	}

	items := adminPromptRowsToItems(rows)
	if err := s.attachAdminPromptTokenTotals(ctx, items); err != nil {
		return AdminPromptListResult{}, err
	}

	return AdminPromptListResult{
		Items:    items,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// attachEstimatedCosts adds optional dashboard-only cost estimates to token aggregates.
// attachPromptTokenTotals fills token totals on prompt history items.
func (s *Service) attachPromptTokenTotals(ctx context.Context, items []PromptHistoryItem) error {
	if len(items) == 0 {
		return nil
	}

	interceptionIDs := make([]string, 0, len(items))
	responseIDs := make([]string, 0, len(items))
	for _, item := range items {
		interceptionIDs = append(interceptionIDs, item.InterceptionID)
		responseIDs = append(responseIDs, item.ProviderResponseID)
	}

	var rows []tokenUsageRow
	if err := s.db.WithContext(ctx).
		Table("token_usages").
		Select("interception_id, provider_response_id, input_tokens, output_tokens").
		Where("interception_id IN ? AND provider_response_id IN ?", interceptionIDs, responseIDs).
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load prompt token totals: %w", err)
	}

	totals := map[string]tokenTotals{}
	for _, row := range rows {
		key := promptTokenKey(row.InterceptionID, row.ProviderResponseID)
		current := totals[key]
		current.Input += row.InputTokens
		current.Output += row.OutputTokens
		totals[key] = current
	}
	for i := range items {
		total := totals[promptTokenKey(items[i].InterceptionID, items[i].ProviderResponseID)]
		items[i].InputTokens = total.Input
		items[i].OutputTokens = total.Output
		items[i].TotalTokens = total.Input + total.Output
	}
	return nil
}

// attachAdminPromptTokenTotals fills token totals on admin prompt history items.
func (s *Service) attachAdminPromptTokenTotals(ctx context.Context, items []AdminPromptHistoryItem) error {
	if len(items) == 0 {
		return nil
	}

	interceptionIDs := make([]string, 0, len(items))
	responseIDs := make([]string, 0, len(items))
	for _, item := range items {
		interceptionIDs = append(interceptionIDs, item.InterceptionID)
		responseIDs = append(responseIDs, item.ProviderResponseID)
	}

	var rows []tokenUsageRow
	if err := s.db.WithContext(ctx).
		Table("token_usages").
		Select("interception_id, provider_response_id, input_tokens, output_tokens").
		Where("interception_id IN ? AND provider_response_id IN ?", interceptionIDs, responseIDs).
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load admin prompt token totals: %w", err)
	}

	totals := map[string]tokenTotals{}
	for _, row := range rows {
		key := promptTokenKey(row.InterceptionID, row.ProviderResponseID)
		current := totals[key]
		current.Input += row.InputTokens
		current.Output += row.OutputTokens
		totals[key] = current
	}
	for i := range items {
		total := totals[promptTokenKey(items[i].InterceptionID, items[i].ProviderResponseID)]
		items[i].InputTokens = total.Input
		items[i].OutputTokens = total.Output
		items[i].TotalTokens = total.Input + total.Output
	}
	return nil
}

// promptSortNeedsTokenTotals reports whether sorting requires the token totals join.
func promptSortNeedsTokenTotals(sortBy string) bool {
	return sortBy == "inputTokens" || sortBy == "outputTokens" || sortBy == "totalTokens"
}

// promptTokenTotalsJoin returns the SQL join used for prompt token aggregate sorting.
func promptTokenTotalsJoin() string {
	return `LEFT JOIN (
		SELECT interception_id,
			provider_response_id,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(input_tokens + output_tokens), 0) AS total_tokens
		FROM token_usages
		GROUP BY interception_id, provider_response_id
	) AS prompt_token_totals
	ON prompt_token_totals.interception_id = user_prompts.interception_id
	AND prompt_token_totals.provider_response_id = user_prompts.provider_response_id`
}

// applyPromptSort applies a validated prompt history order to the query.
func applyPromptSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"prompt":       "user_prompts.prompt",
		"provider":     "interceptions.provider",
		"model":        "interceptions.model",
		"createdAt":    "user_prompts.created_at",
		"durationMs":   durationSortExpression(query),
		"inputTokens":  "COALESCE(prompt_token_totals.input_tokens, 0)",
		"outputTokens": "COALESCE(prompt_token_totals.output_tokens, 0)",
		"totalTokens":  "COALESCE(prompt_token_totals.total_tokens, 0)",
	}

	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	if sortBy == "durationMs" {
		return applyDurationSort(query, column, dir), nil
	}
	return query.Order(column + " " + dir).Order("user_prompts.id ASC"), nil
}

// applyAdminPromptSort applies a validated admin prompt history order to the query.
func applyAdminPromptSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"prompt":       "user_prompts.prompt",
		"provider":     "interceptions.provider",
		"model":        "interceptions.model",
		"createdAt":    "user_prompts.created_at",
		"durationMs":   durationSortExpression(query),
		"inputTokens":  "COALESCE(prompt_token_totals.input_tokens, 0)",
		"outputTokens": "COALESCE(prompt_token_totals.output_tokens, 0)",
		"totalTokens":  "COALESCE(prompt_token_totals.total_tokens, 0)",
		"userName":     "users.name",
		"userEmail":    "users.email",
	}

	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	if sortBy == "durationMs" {
		return applyDurationSort(query, column, dir), nil
	}
	return query.Order(column + " " + dir).Order("user_prompts.id ASC"), nil
}

// applyDurationSort orders completed interceptions by duration and leaves pending rows last.
func applyDurationSort(query *gorm.DB, column, dir string) *gorm.DB {
	return query.
		Order("CASE WHEN interceptions.ended_at IS NULL OR interceptions.ended_at < interceptions.started_at THEN 1 ELSE 0 END ASC").
		Order(column + " " + dir).
		Order("user_prompts.id ASC")
}

// durationSortExpression returns a dialect-aware millisecond duration expression.
func durationSortExpression(query *gorm.DB) string {
	if query.Dialector.Name() == "sqlite" {
		return "((julianday(interceptions.ended_at) - julianday(interceptions.started_at)) * 86400000)"
	}

	return "EXTRACT(EPOCH FROM (interceptions.ended_at - interceptions.started_at)) * 1000"
}

// normalizeSortDir converts a prompt sort direction into SQL syntax.
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

// promptRowsToItems maps database prompt rows into API items.
func promptRowsToItems(rows []promptRow) []PromptHistoryItem {
	items := make([]PromptHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, PromptHistoryItem{
			ID:                 row.ID,
			InterceptionID:     row.InterceptionID,
			ProviderResponseID: row.ProviderResponseID,
			Provider:           row.Provider,
			ProviderType:       row.ProviderType,
			Model:              row.Model,
			Prompt:             row.Prompt,
			DurationMs:         durationMilliseconds(row.StartedAt, row.EndedAt),
			CreatedAt:          row.CreatedAt,
		})
	}
	return items
}

// adminPromptRowsToItems maps admin database prompt rows into API items.
func adminPromptRowsToItems(rows []adminPromptRow) []AdminPromptHistoryItem {
	items := make([]AdminPromptHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, AdminPromptHistoryItem{
			ID:                    row.ID,
			InterceptionID:        row.InterceptionID,
			ProviderResponseID:    row.ProviderResponseID,
			Provider:              row.Provider,
			ProviderType:          row.ProviderType,
			Model:                 row.Model,
			Prompt:                row.Prompt,
			UserID:                row.UserID,
			UserName:              row.UserName,
			UserEmail:             row.UserEmail,
			UserPreferredUsername: row.UserPreferredUsername,
			ClientIP:              row.ClientIP,
			DurationMs:            durationMilliseconds(row.StartedAt, row.EndedAt),
			CreatedAt:             row.CreatedAt,
		})
	}
	return items
}

// durationMilliseconds returns a completed interception duration in milliseconds.
func durationMilliseconds(startedAt time.Time, endedAt *time.Time) *int64 {
	if startedAt.IsZero() || endedAt == nil || endedAt.Before(startedAt) {
		return nil
	}

	duration := endedAt.Sub(startedAt).Milliseconds()
	return &duration
}
