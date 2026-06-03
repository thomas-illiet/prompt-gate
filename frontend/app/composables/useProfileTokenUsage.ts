import type { DailyUsage, DashboardActivityResponse } from '~/types/user-service'
import { toApiErrorMessage } from '~/utils/api-error'

const PROFILE_TOKEN_HISTORY_DAYS = 365
const DAY_MS = 24 * 60 * 60 * 1000
const DATE_KEY_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/

const ERROR_MESSAGES = {
  invalid_usage_window: 'Usage window must be 7 days, 30 days, or all time.',
}

export interface ProfileTokenUsageDay extends DailyUsage {
  date: string
}

export interface ProfileTokenUsageSummary {
  activeDays: number
  currentStreakDays: number
  days: ProfileTokenUsageDay[]
  endsAt: string
  longestStreakDays: number
  maxTokens: number
  peakDay: ProfileTokenUsageDay | null
  startsAt: string
  totalTokens: number
}

// toProfileTokenUsageErrorMessage converts profile usage API errors into user-facing text.
export function toProfileTokenUsageErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected profile token activity error.',
  )
}

// buildProfileTokenUsageSummary normalizes dashboard activity into the profile heatmap window.
export function buildProfileTokenUsageSummary(
  response: DashboardActivityResponse | null,
  fallbackEndDate = new Date(),
): ProfileTokenUsageSummary {
  const endDate = parseDateKey(resolveEndDateKey(response, fallbackEndDate))
  const startDate = addUtcDays(endDate, -(PROFILE_TOKEN_HISTORY_DAYS - 1))
  const usageByDate = new Map(
    (response?.daily ?? []).map((day) => [day.date, day] as const),
  )
  const days = Array.from({ length: PROFILE_TOKEN_HISTORY_DAYS }, (_, index) =>
    profileTokenUsageDay(
      utcDateKey(addUtcDays(startDate, index)),
      usageByDate,
    ),
  )

  let currentStreakDays = 0
  let longestStreakDays = 0
  let runningStreakDays = 0
  let peakDay: ProfileTokenUsageDay | null = null
  let totalTokens = 0
  let activeDays = 0
  let maxTokens = 0

  for (const day of days) {
    totalTokens += day.totalTokens
    if (day.totalTokens > 0) {
      activeDays += 1
      runningStreakDays += 1
      longestStreakDays = Math.max(longestStreakDays, runningStreakDays)
    } else {
      runningStreakDays = 0
    }

    if (day.totalTokens > maxTokens) {
      maxTokens = day.totalTokens
      peakDay = day
    }
  }

  for (let index = days.length - 1; index >= 0; index -= 1) {
    if (days[index]?.totalTokens === 0) {
      break
    }
    currentStreakDays += 1
  }

  return {
    activeDays,
    currentStreakDays,
    days,
    endsAt: utcDateKey(endDate),
    longestStreakDays,
    maxTokens,
    peakDay,
    startsAt: utcDateKey(startDate),
    totalTokens,
  }
}

// useProfileTokenUsage loads the current user's profile token activity.
export function useProfileTokenUsage() {
  const apiFetch = useApiFetch()

  const activity = shallowRef<DashboardActivityResponse | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const summary = computed(() => buildProfileTokenUsageSummary(activity.value))

  async function loadUsage() {
    loading.value = true
    error.value = null

    try {
      activity.value = await apiFetch<DashboardActivityResponse>(
        '/api/v1/me/dashboard/activity?window=all',
      )
    } catch (fetchError) {
      error.value = toProfileTokenUsageErrorMessage(fetchError)
    } finally {
      loading.value = false
    }
  }

  async function reload() {
    await loadUsage()
  }

  void loadUsage()

  return {
    activity,
    error,
    loading,
    reload,
    summary,
  }
}

function profileTokenUsageDay(
  date: string,
  usageByDate: Map<string, DailyUsage>,
): ProfileTokenUsageDay {
  const day = usageByDate.get(date)

  return {
    date,
    requests: day?.requests ?? 0,
    prompts: day?.prompts ?? 0,
    inputTokens: day?.inputTokens ?? 0,
    outputTokens: day?.outputTokens ?? 0,
    completionInputTokens: day?.completionInputTokens ?? 0,
    completionOutputTokens: day?.completionOutputTokens ?? 0,
    completionTokens: day?.completionTokens ?? 0,
    embeddingTokens: day?.embeddingTokens ?? 0,
    totalTokens: day?.totalTokens ?? 0,
    ...(day?.estimatedCost ? { estimatedCost: day.estimatedCost } : {}),
  }
}

function resolveEndDateKey(
  response: DashboardActivityResponse | null,
  fallbackEndDate: Date,
) {
  const responseEndDate = dateKeyFromValue(response?.endsAt)
  if (responseEndDate) {
    return responseEndDate
  }

  const latestDailyDate = [...(response?.daily ?? [])]
    .map((day) => day.date)
    .filter((date) => DATE_KEY_PATTERN.test(date))
    .sort()
    .at(-1)

  return latestDailyDate ?? utcDateKey(fallbackEndDate)
}

function dateKeyFromValue(value: string | null | undefined) {
  if (!value) {
    return ''
  }
  if (DATE_KEY_PATTERN.test(value)) {
    return value
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }

  return utcDateKey(date)
}

function parseDateKey(value: string) {
  const match = DATE_KEY_PATTERN.exec(value)
  if (!match) {
    return new Date(Number.NaN)
  }

  return new Date(
    Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])),
  )
}

function utcDateKey(date: Date) {
  return date.toISOString().slice(0, 10)
}

function addUtcDays(date: Date, days: number) {
  return new Date(date.getTime() + days * DAY_MS)
}
