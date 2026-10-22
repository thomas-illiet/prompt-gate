import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, mount } from '@vue/test-utils'
import { FetchError } from 'ofetch'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  toProfileQuotaResetErrorMessage,
  useProfileQuotaResets,
} from '../../app/composables/useProfileQuotaResets'
import type {
  QuotaResetHistoryResponse,
  QuotaResetSummary,
} from '../../app/types/quota-resets'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return {
    apiFetch,
    useApiFetchMock: vi.fn(() => apiFetch),
  }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

const availableSummary: QuotaResetSummary = {
  available: 1,
  maximum: 2,
  hasResettableQuota: true,
  pendingReset: false,
}

const historyResponse: QuotaResetHistoryResponse = {
  items: [
    {
      id: 'monthly-event',
      type: 'monthly_grant',
      delta: 1,
      balanceAfter: 1,
      createdAt: '2026-10-01T00:00:00Z',
    },
  ],
  page: 1,
  pageSize: 20,
  total: 1,
}

function apiError(code: string) {
  return Object.assign(Object.create(FetchError.prototype), {
    response: {
      _data: { error: code },
    },
  }) as FetchError
}

function mountProbe() {
  const holder = {} as { state: ReturnType<typeof useProfileQuotaResets> }
  const Probe = defineComponent({
    setup() {
      holder.state = useProfileQuotaResets()
      return () => null
    },
  })

  const wrapper = mount(Probe)
  return { state: holder.state, wrapper }
}

describe('useProfileQuotaResets', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads the summary immediately and keeps history lazy and paginated', async () => {
    apiFetch
      .mockResolvedValueOnce(availableSummary)
      .mockResolvedValueOnce(historyResponse)
      .mockResolvedValueOnce({
        ...historyResponse,
        items: [],
        page: 2,
        total: 11,
      })

    const { state, wrapper } = mountProbe()
    await flushPromises()

    expect(apiFetch).toHaveBeenCalledTimes(1)
    expect(apiFetch).toHaveBeenNthCalledWith(1, '/api/v1/me/quota-resets')
    expect(state.summary.value).toEqual(availableSummary)

    await state.openHistory()

    expect(state.historyOpen.value).toBe(true)
    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/me/quota-resets/history?page=1&pageSize=20',
    )
    expect(state.history.value).toEqual(historyResponse.items)

    await state.setHistoryPage(2)

    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      '/api/v1/me/quota-resets/history?page=2&pageSize=20',
    )
    expect(state.historyPage.value).toBe(2)
    wrapper.unmount()
  })

  it('uses a reset with an idempotency key and applies the returned summary', async () => {
    const usedSummary = { ...availableSummary, available: 0 }
    apiFetch.mockResolvedValueOnce(availableSummary).mockResolvedValueOnce({
      status: 'applied',
      summary: usedSummary,
      quota: {},
    })

    const { state, wrapper } = mountProbe()
    await flushPromises()

    const response = await state.useReset()

    expect(response?.status).toBe('applied')
    expect(apiFetch).toHaveBeenNthCalledWith(2, '/api/v1/me/quota-resets/use', {
      method: 'POST',
      headers: { 'Idempotency-Key': expect.any(String) },
    })
    expect(state.summary.value).toEqual(usedSummary)
    wrapper.unmount()
  })

  it('reuses the idempotency key when reset consumption is retried', async () => {
    const retryError = new Error('connection interrupted')
    apiFetch
      .mockResolvedValueOnce(availableSummary)
      .mockRejectedValueOnce(retryError)
      .mockResolvedValueOnce({
        status: 'applied',
        summary: { ...availableSummary, available: 0 },
        quota: {},
      })

    const { state, wrapper } = mountProbe()
    await flushPromises()

    await expect(state.useReset()).rejects.toBe(retryError)
    await state.useReset()

    const firstHeaders = apiFetch.mock.calls[1]?.[1]?.headers
    const retryHeaders = apiFetch.mock.calls[2]?.[1]?.headers
    expect(firstHeaders).toEqual({ 'Idempotency-Key': expect.any(String) })
    expect(retryHeaders).toEqual(firstHeaders)
    wrapper.unmount()
  })

  it('polls pending resets every five seconds and refreshes open history once applied', async () => {
    const pendingSummary = { ...availableSummary, pendingReset: true }
    const appliedSummary = {
      ...availableSummary,
      available: 0,
      pendingReset: false,
    }
    apiFetch
      .mockResolvedValueOnce(availableSummary)
      .mockResolvedValueOnce(historyResponse)
      .mockResolvedValueOnce({
        status: 'pending',
        summary: pendingSummary,
        quota: {},
      })
      .mockResolvedValueOnce(appliedSummary)
      .mockResolvedValueOnce({
        ...historyResponse,
        items: [
          ...historyResponse.items,
          {
            id: 'used-event',
            type: 'quota_reset_used',
            delta: -1,
            balanceAfter: 0,
            createdAt: '2026-10-02T00:00:00Z',
          },
        ],
        total: 2,
      })

    const { state, wrapper } = mountProbe()
    await flushPromises()
    await state.openHistory()
    await state.useReset()

    await vi.advanceTimersByTimeAsync(4_999)
    expect(apiFetch).toHaveBeenCalledTimes(3)

    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()

    expect(apiFetch).toHaveBeenNthCalledWith(4, '/api/v1/me/quota-resets')
    expect(apiFetch).toHaveBeenNthCalledWith(
      5,
      '/api/v1/me/quota-resets/history?page=1&pageSize=20',
    )
    expect(state.summary.value).toEqual(appliedSummary)
    expect(state.historyTotal.value).toBe(2)

    await vi.advanceTimersByTimeAsync(10_000)
    expect(apiFetch).toHaveBeenCalledTimes(5)
    wrapper.unmount()
  })

  it('keeps history lazy when a pending reset completes while it is closed', async () => {
    apiFetch
      .mockResolvedValueOnce(availableSummary)
      .mockResolvedValueOnce({
        status: 'pending',
        summary: { ...availableSummary, pendingReset: true },
        quota: {},
      })
      .mockResolvedValueOnce({
        ...availableSummary,
        available: 0,
        pendingReset: false,
      })

    const { state, wrapper } = mountProbe()
    await flushPromises()
    await state.useReset()
    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()

    expect(apiFetch).toHaveBeenCalledTimes(3)
    expect(
      apiFetch.mock.calls.some(([path]) => String(path).includes('/history')),
    ).toBe(false)
    wrapper.unmount()
  })

  it('refreshes and polls when another reset is already pending', async () => {
    const pendingError = apiError('quota_reset_pending')
    const pendingSummary = { ...availableSummary, pendingReset: true }
    const appliedSummary = {
      ...availableSummary,
      available: 0,
      pendingReset: false,
    }
    apiFetch
      .mockResolvedValueOnce(availableSummary)
      .mockRejectedValueOnce(pendingError)
      .mockResolvedValueOnce(pendingSummary)
      .mockResolvedValueOnce(appliedSummary)

    const { state, wrapper } = mountProbe()
    await flushPromises()

    await expect(state.useReset()).rejects.toBe(pendingError)

    expect(apiFetch).toHaveBeenNthCalledWith(3, '/api/v1/me/quota-resets')
    expect(state.summary.value).toEqual(pendingSummary)

    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()

    expect(apiFetch).toHaveBeenNthCalledWith(4, '/api/v1/me/quota-resets')
    expect(state.summary.value).toEqual(appliedSummary)
    wrapper.unmount()
  })

  it('does not restart pending polling after the composable is disposed', async () => {
    let resolvePoll: ((summary: QuotaResetSummary) => void) | undefined
    const pollResponse = new Promise<QuotaResetSummary>((resolve) => {
      resolvePoll = resolve
    })
    apiFetch
      .mockResolvedValueOnce({ ...availableSummary, pendingReset: true })
      .mockReturnValueOnce(pollResponse)

    const { wrapper } = mountProbe()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5_000)

    expect(apiFetch).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    resolvePoll?.({ ...availableSummary, pendingReset: true })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(10_000)

    expect(apiFetch).toHaveBeenCalledTimes(2)
  })

  it('maps reset business errors to readable messages', () => {
    expect(
      toProfileQuotaResetErrorMessage(
        apiError('quota_reset_credit_unavailable'),
      ),
    ).toBe('No quota reset is currently available.')
    expect(
      toProfileQuotaResetErrorMessage(apiError('quota_reset_not_applicable')),
    ).toBe('Your current quota cannot be reset.')
    expect(
      toProfileQuotaResetErrorMessage(apiError('quota_reset_pending')),
    ).toBe('A quota reset is already pending.')
    expect(
      toProfileQuotaResetErrorMessage(apiError('missing_idempotency_key')),
    ).toBe('Unable to safely submit the reset. Try again.')
  })
})
