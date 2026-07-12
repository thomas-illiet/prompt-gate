package proxy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// usageWindowForDays converts legacy day counts into dashboard usage windows.
func usageWindowForDays(days int) (UsageWindow, error) {
	switch days {
	case 7:
		return UsageWindow7Days, nil
	case 30:
		return UsageWindow30Days, nil
	default:
		return "", ErrInvalidUsageWindow
	}
}

// resolveUsageWindow resolves a current-user usage window into concrete timestamps.
func (s *Service) resolveUsageWindow(ctx context.Context, userID string, window UsageWindow, now time.Time) (usageRange, error) {
	return s.resolveDashboardWindow(ctx, currentUserDashboardScope(userID), window, now)
}

// resolveDashboardWindow resolves a dashboard window into concrete UTC boundaries.
func (s *Service) resolveDashboardWindow(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time) (usageRange, error) {
	if now.IsZero() {
		now = time.Now()
	}
	endsAt := now.UTC()

	switch window {
	case "":
		window = UsageWindow30Days
	case UsageWindow7Days, UsageWindow30Days, UsageWindowAll:
	default:
		return usageRange{}, ErrInvalidUsageWindow
	}

	if window == UsageWindowAll {
		firstActivityAt, ok, err := s.firstActivityAt(ctx, scope)
		if err != nil {
			return usageRange{}, err
		}
		if !ok {
			return usageRange{
				UsageWindowMeta: UsageWindowMeta{
					Window:   UsageWindowAll,
					StartsAt: endsAt,
					EndsAt:   endsAt,
				},
			}, nil
		}

		startsAt := dayStart(firstActivityAt)
		days := daysBetween(startsAt, dayStart(endsAt)) + 1
		if days < 1 {
			days = 1
		}
		return usageRange{
			UsageWindowMeta: UsageWindowMeta{
				Window:   UsageWindowAll,
				StartsAt: startsAt,
				EndsAt:   endsAt,
			},
			Days: days,
		}, nil
	}

	days := 30
	if window == UsageWindow7Days {
		days = 7
	}
	startsAt := dayStart(endsAt).AddDate(0, 0, -(days - 1))
	return usageRange{
		UsageWindowMeta: UsageWindowMeta{
			Window:   window,
			StartsAt: startsAt,
			EndsAt:   endsAt,
		},
		Days: days,
	}, nil
}

// firstActivityAt returns the earliest recorded request for a dashboard scope.
func (s *Service) firstActivityAt(ctx context.Context, scope dashboardUsageScope) (time.Time, bool, error) {
	var row struct {
		Day time.Time
	}
	query := s.db.WithContext(ctx).
		Model(&ProxyDailyUsageKPI{}).
		Select("day")
	query = scope.applyInitiatorFilter(query, "initiator_id").
		Order("day ASC")
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("load first activity: %w", err)
	}
	return row.Day, true, nil
}

// buildDailyBuckets creates empty daily usage buckets for a window.
func buildDailyBuckets(startsAt time.Time, days int) []DailyUsage {
	buckets := make([]DailyUsage, 0, days)
	for i := 0; i < days; i++ {
		buckets = append(buckets, DailyUsage{Date: startsAt.AddDate(0, 0, i).Format("2006-01-02")})
	}
	return buckets
}

// breakdown returns an existing or new usage breakdown for a display name.
func breakdown(values map[string]*UsageBreakdown, name string) *UsageBreakdown {
	return breakdownByKey(values, name, name)
}

// breakdownByKey returns an existing or new usage breakdown for a stable key.
func breakdownByKey(values map[string]*UsageBreakdown, key, name string) *UsageBreakdown {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = name
	}
	if values[key] == nil {
		values[key] = &UsageBreakdown{Name: name, key: key}
	}
	return values[key]
}

// sortedBreakdowns returns the highest-volume usage breakdowns.
func sortedBreakdowns(values map[string]*UsageBreakdown, limit int) []UsageBreakdown {
	items := make([]UsageBreakdown, 0, len(values))
	for _, value := range values {
		if value.Requests == 0 && value.TotalTokens == 0 {
			continue
		}
		items = append(items, *value)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalTokens == items[j].TotalTokens {
			if items[i].Requests == items[j].Requests {
				return items[i].Name < items[j].Name
			}
			return items[i].Requests > items[j].Requests
		}
		return items[i].TotalTokens > items[j].TotalTokens
	})
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

// dateKey formats a timestamp as a UTC daily bucket key.
func dateKey(value time.Time) string {
	return value.UTC().Format("2006-01-02")
}

// dayStart returns the UTC midnight for a timestamp.
func dayStart(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// daysBetween returns the whole-day distance between two UTC-normalized dates.
func daysBetween(start, end time.Time) int {
	return int(dayStart(end).Sub(dayStart(start)).Hours() / 24)
}

// promptTokenKey builds a collision-safe key for prompt token totals.
func promptTokenKey(interceptionID, responseID string) string {
	return interceptionID + "\x00" + responseID
}
