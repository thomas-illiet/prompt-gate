import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import {
  toUserUsageErrorMessage,
  useUserUsage,
} from '../../app/composables/useUserUsage'
import type { UserUsageSummary } from '../../app/types/user-service'

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

function usageSummary(days: 7 | 30): UserUsageSummary {
  return {
    days,
    startsAt: '2026-01-01T00:00:00Z',
    endsAt: '2026-01-30T00:00:00Z',
    totals: {
      requests: days,
      prompts: 2,
      toolCalls: 1,
      inputTokens: 10,
      outputTokens: 20,
      cacheReadInputTokens: 3,
      cacheWriteInputTokens: 4,
      completionInputTokens: 17,
      completionOutputTokens: 20,
      completionTokens: 37,
      embeddingTokens: 0,
      totalTokens: 37,
    },
    daily: [],
    topModels: [],
    topProviders: [],
    recentPrompts: [],
  }
}

describe('useUserUsage', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  it('loads 30 day usage by default and reloads when window changes', async () => {
    apiFetch
      .mockResolvedValueOnce(usageSummary(30))
      .mockResolvedValueOnce(usageSummary(7))

    const userUsage = useUserUsage()
    await vi.waitFor(() => expect(userUsage.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenNthCalledWith(1, '/api/v1/me/usage?days=30')
    expect(userUsage.usage.value?.days).toBe(30)

    userUsage.setDays(7)

    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenNthCalledWith(2, '/api/v1/me/usage?days=7'),
    )
    expect(userUsage.usage.value?.days).toBe(7)
  })

  it('maps API errors to readable messages', () => {
    expect(toUserUsageErrorMessage(apiError('invalid_usage_window'))).toBe(
      'Usage window must be 7 days, 30 days, or all time.',
    )
  })
})
