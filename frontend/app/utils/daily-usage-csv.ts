import type { DailyUsage, DashboardMetric } from '~/types/user-service'
import { downloadCsv, rowsToCsv } from './csv'

export type DailyUsageCsvView = DashboardMetric | 'annual'

const METRIC_HEADERS: Record<DashboardMetric, string> = {
  requests: 'API requests',
  tokens: 'Tokens',
  cost: 'Estimated cost (USD)',
}

function metricValue(day: DailyUsage, view: DashboardMetric) {
  if (view === 'requests') return day.requests
  if (view === 'tokens') return day.totalTokens
  return day.estimatedCost?.totalUsd ?? null
}

// dailyUsageToCsv serializes the columns displayed by a dashboard daily-data view.
export function dailyUsageToCsv(
  days: readonly DailyUsage[],
  view: DailyUsageCsvView,
) {
  if (view === 'annual') {
    return rowsToCsv([
      ['Date', 'API requests', 'Tokens'],
      ...days.map((day) => [day.date, day.requests, day.totalTokens]),
    ])
  }

  return rowsToCsv([
    ['Date', METRIC_HEADERS[view]],
    ...days.map((day) => [day.date, metricValue(day, view)]),
  ])
}

// downloadDailyUsageCsv exports dashboard daily data with spreadsheet-safe encoding.
export function downloadDailyUsageCsv(
  days: readonly DailyUsage[],
  view: DailyUsageCsvView,
  filename: string,
) {
  downloadCsv(dailyUsageToCsv(days, view), filename)
}
