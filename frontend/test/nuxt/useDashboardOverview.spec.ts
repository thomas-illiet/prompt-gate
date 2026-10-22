import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, shallowRef } from 'vue'

import {
  clearDashboardOverviewCache,
  useDashboardOverview,
} from '../../app/composables/useDashboardOverview'
import type {
  DashboardCockpitOverviewResponse,
  DashboardScope,
  UsageWindow,
} from '../../app/types/user-service'

const { apiFetch, authUser, useApiFetchMock, useAuthStoreMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  const authUser = { value: { id: 'user-1' } }
  return {
    apiFetch,
    authUser,
    useApiFetchMock: vi.fn(() => apiFetch),
    useAuthStoreMock: vi.fn(() => ({
      get user() {
        return authUser.value
      },
    })),
  }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)
mockNuxtImport('useAuthStore', () => useAuthStoreMock)

function response(
  window: UsageWindow,
  requests: number,
): DashboardCockpitOverviewResponse {
  return {
    window,
    startsAt: '2026-10-01T00:00:00Z',
    endsAt: '2026-10-08T00:00:00Z',
    totals: {
      requests,
      toolCalls: 0,
      inputTokens: requests,
      outputTokens: 0,
      cacheReadInputTokens: 0,
      cacheWriteInputTokens: 0,
      completionInputTokens: requests,
      completionOutputTokens: 0,
      completionTokens: requests,
      embeddingTokens: 0,
      totalTokens: requests,
    },
    totalDurationMs: 0,
    averageDurationMs: requests ? 0 : null,
    daily: [],
    costCoverage: { status: 'unavailable' },
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, reject, resolve }
}

describe('useDashboardOverview', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    useAuthStoreMock.mockClear()
    authUser.value = { id: 'user-1' }
    clearDashboardOverviewCache()
  })

  it('uses the aggregated self endpoint and keeps same-key data on refresh failure', async () => {
    apiFetch.mockResolvedValueOnce(response('7d', 7))
    const scope = shallowRef<DashboardScope>('self')
    const window = shallowRef<UsageWindow>('7d')
    const overview = useDashboardOverview(scope, window)

    await vi.waitFor(() => expect(overview.data.value?.totals.requests).toBe(7))
    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/me/dashboard/overview?window=7d',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )

    apiFetch.mockRejectedValueOnce(new Error('offline'))
    await overview.reload()

    expect(overview.data.value?.totals.requests).toBe(7)
    expect(overview.error.value).toBeNull()
    expect(overview.staleWarning.value).toContain(
      'Update failed — showing data from',
    )
  })

  it('aborts obsolete requests and never shows data from a previous key', async () => {
    const first = deferred<DashboardCockpitOverviewResponse>()
    const second = deferred<DashboardCockpitOverviewResponse>()
    apiFetch.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const scope = shallowRef<DashboardScope>('self')
    const window = shallowRef<UsageWindow>('7d')
    const overview = useDashboardOverview(scope, window)
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(1))
    const firstSignal = apiFetch.mock.calls[0]?.[1]?.signal as AbortSignal

    window.value = '30d'
    await nextTick()

    expect(firstSignal.aborted).toBe(true)
    expect(overview.data.value).toBeNull()
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2))

    first.resolve(response('7d', 70))
    await nextTick()
    expect(overview.data.value).toBeNull()

    second.resolve(response('30d', 30))
    await vi.waitFor(() => expect(overview.data.value?.window).toBe('30d'))
  })

  it('switches to the admin aggregate endpoint without leaking self data', async () => {
    apiFetch
      .mockResolvedValueOnce(response('7d', 2))
      .mockRejectedValueOnce(new Error('admin unavailable'))
    const scope = shallowRef<DashboardScope>('self')
    const window = shallowRef<UsageWindow>('7d')
    const overview = useDashboardOverview(scope, window)
    await vi.waitFor(() => expect(overview.data.value?.totals.requests).toBe(2))

    scope.value = 'global'
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2))
    await vi.waitFor(() => expect(overview.error.value).not.toBeNull())

    expect(apiFetch.mock.calls[1]?.[0]).toBe(
      '/api/v1/admin/dashboard/overview?window=7d',
    )
    expect(overview.data.value).toBeNull()
    expect(overview.staleWarning.value).toBeNull()
  })
})
