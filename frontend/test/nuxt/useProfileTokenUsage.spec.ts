import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { FetchError } from 'ofetch'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  buildProfileTokenUsageSummary,
  toProfileTokenUsageErrorMessage,
  useProfileTokenUsage,
} from '../../app/composables/useProfileTokenUsage'
import type {
  DailyUsage,
  DashboardActivityResponse,
} from '../../app/types/user-service'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return {
    apiFetch,
    useApiFetchMock: vi.fn(() => apiFetch),
  }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

function apiError(code: string) {
  return Object.assign(Object.create(FetchError.prototype), {
    response: {
      _data: { error: code },
    },
  }) as FetchError
}

function dailyUsage(
  date: string,
  totalTokens: number,
  requests = totalTokens > 0 ? 1 : 0,
): DailyUsage {
  return {
    date,
    requests,
    prompts: requests,
    inputTokens: totalTokens,
    outputTokens: 0,
    completionInputTokens: totalTokens,
    completionOutputTokens: 0,
    completionTokens: totalTokens,
    embeddingTokens: 0,
    totalTokens,
  }
}

function activityResponse(
  daily: DailyUsage[],
  endsAt = '2026-06-03T12:00:00Z',
): DashboardActivityResponse {
  return {
    window: 'all',
    startsAt: '2025-01-01T00:00:00Z',
    endsAt,
    daily,
  }
}

describe('useProfileTokenUsage', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
  })

  it('loads all-time activity and derives the 365 day profile summary', async () => {
    apiFetch.mockResolvedValueOnce(
      activityResponse([
        dailyUsage('2024-01-01', 999),
        dailyUsage('2025-06-04', 4),
        dailyUsage('2026-06-01', 11),
        dailyUsage('2026-06-02', 12),
        dailyUsage('2026-06-03', 0),
      ]),
    )

    const profileUsage = useProfileTokenUsage()
    await vi.waitFor(() => expect(profileUsage.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenCalledWith(
      '/api/v1/me/dashboard/activity?window=all',
    )
    expect(profileUsage.summary.value.days).toHaveLength(365)
    expect(profileUsage.summary.value.startsAt).toBe('2025-06-04')
    expect(profileUsage.summary.value.endsAt).toBe('2026-06-03')
    expect(profileUsage.summary.value.totalTokens).toBe(27)
    expect(profileUsage.summary.value.activeDays).toBe(3)
    expect(profileUsage.summary.value.currentStreakDays).toBe(0)
    expect(profileUsage.summary.value.longestStreakDays).toBe(2)
    expect(profileUsage.summary.value.peakDay?.date).toBe('2026-06-02')
    expect(
      profileUsage.summary.value.days.find((day) => day.date === '2026-05-31')
        ?.totalTokens,
    ).toBe(0)
  })

  it('calculates the current streak when the last visible day is active', () => {
    const summary = buildProfileTokenUsageSummary(
      activityResponse([
        dailyUsage('2026-05-30', 3),
        dailyUsage('2026-06-01', 11),
        dailyUsage('2026-06-02', 12),
        dailyUsage('2026-06-03', 13),
      ]),
    )

    expect(summary.currentStreakDays).toBe(3)
    expect(summary.longestStreakDays).toBe(3)
    expect(summary.totalTokens).toBe(39)
  })

  it('maps API errors to readable messages', () => {
    expect(toProfileTokenUsageErrorMessage(apiError('invalid_usage_window'))).toBe(
      'Usage window must be 7 days, 30 days, or all time.',
    )
  })
})
