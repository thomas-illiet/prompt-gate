package proxy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"

	"gorm.io/gorm"
)

var accountIPSortColumns = map[string]string{
	"ip":       "ip",
	"lastSeen": "last_seen",
}

// ListAccountIPAddresses returns the client IP addresses observed for one account.
func (s *Service) ListAccountIPAddresses(ctx context.Context, userID string, userType auth.UserType, params AccountIPAddressListParams) (AccountIPAddressListResult, error) {
	var account users.User
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", userID, userType).Take(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AccountIPAddressListResult{}, users.ErrUserNotFound
		}
		return AccountIPAddressListResult{}, fmt.Errorf("find account for IP addresses: %w", err)
	}

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	column, ok := accountIPSortColumns[params.SortBy]
	if !ok {
		return AccountIPAddressListResult{}, ErrInvalidSort
	}
	direction := strings.ToLower(params.SortDir)
	if direction != "asc" && direction != "desc" {
		return AccountIPAddressListResult{}, ErrInvalidSort
	}

	query := s.db.WithContext(ctx).Model(&AccountIPAddress{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return AccountIPAddressListResult{}, fmt.Errorf("count account IP addresses: %w", err)
	}
	items := make([]AccountIPAddress, 0)
	if err := query.Order(column + " " + direction).
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&items).Error; err != nil {
		return AccountIPAddressListResult{}, fmt.Errorf("list account IP addresses: %w", err)
	}
	return AccountIPAddressListResult{Items: items, Page: params.Page, PageSize: params.PageSize, Total: total}, nil
}
