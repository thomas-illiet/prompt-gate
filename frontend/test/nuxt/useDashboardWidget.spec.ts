import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { shallowRef } from 'vue'

import {
  toDashboardWidgetErrorMessage,
  useDashboardWidget,
} from '../../app/composables/useDashboardWidget'
import type {
  DashboardTokensResponse,
  UsageWindow,
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

function tokensResponse(
  window: UsageWindow,
  totalTokens: number,
): DashboardTokensResponse {
  return {
    window,
    startsAt: '2026-01-01T00:00:00Z',
    endsAt: '2026-01-30T00:00:00Z',
    inputTokens: totalTokens,
    outputTokens: 0,
    cacheReadInputTokens: 0,
    cacheWriteInputTokens: 0,
    completionInputTokens: totalTokens,
    completionOutputTokens: 0,
    completionTokens: totalTokens,
    embeddingTokens: 0,
    totalTokens,
  }
}

describe('useDashboardWidget', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
  })

  it('loads one widget and reloads when the window changes', async () => {
    apiFetch
      .mockResolvedValueOnce(tokensResponse('30d', 30))
      .mockResolvedValueOnce(tokensResponse('7d', 7))
    const window = shallowRef<UsageWindow>('30d')

    const widget = useDashboardWidget<DashboardTokensResponse>(
      '/api/v1/me/dashboard/tokens',
      window,
    )

    await vi.waitFor(() => expect(widget.loading.value).toBe(false))
    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/me/dashboard/tokens?window=30d',
    )
    expect(widget.data.value?.totalTokens).toBe(30)

    window.value = '7d'

    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenNthCalledWith(
        2,
        '/api/v1/me/dashboard/tokens?window=7d',
      ),
    )
    await vi.waitFor(() => expect(widget.data.value?.totalTokens).toBe(7))
  })

  it('keeps previous data when a widget refresh fails', async () => {
    apiFetch
      .mockResolvedValueOnce(tokensResponse('30d', 30))
      .mockRejectedValueOnce(apiError('invalid_usage_window'))
    const window = shallowRef<UsageWindow>('30d')

    const widget = useDashboardWidget<DashboardTokensResponse>(
      '/api/v1/me/dashboard/tokens',
      window,
    )
    await vi.waitFor(() => expect(widget.data.value?.totalTokens).toBe(30))

    window.value = 'all'

    await vi.waitFor(() =>
      expect(widget.error.value).toBe(
        'Usage window must be 7 days, 30 days, or all time.',
      ),
    )
    expect(widget.data.value?.totalTokens).toBe(30)
  })

  it('reloads when the endpoint changes', async () => {
    apiFetch
      .mockResolvedValueOnce(tokensResponse('7d', 7))
      .mockResolvedValueOnce(tokensResponse('7d', 70))
    const endpoint = shallowRef('/api/v1/me/dashboard/tokens')
    const window = shallowRef<UsageWindow>('7d')

    const widget = useDashboardWidget<DashboardTokensResponse>(endpoint, window)
    await vi.waitFor(() => expect(widget.data.value?.totalTokens).toBe(7))

    endpoint.value = '/api/v1/admin/dashboard/tokens'

    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenNthCalledWith(
        2,
        '/api/v1/admin/dashboard/tokens?window=7d',
      ),
    )
    await vi.waitFor(() => expect(widget.data.value?.totalTokens).toBe(70))
  })

  it('maps dashboard widget API errors to readable messages', () => {
    expect(
      toDashboardWidgetErrorMessage(apiError('invalid_usage_window')),
    ).toBe('Usage window must be 7 days, 30 days, or all time.')
  })
})
