import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, shallowRef } from 'vue'

import {
  exhaustedQuotaWindows,
  useQuotaExhaustionDialog,
} from '../../app/composables/useQuotaExhaustionDialog'
import type {
  QuotaResetSummary,
  UseQuotaResetResponse,
} from '../../app/types/quota-resets'
import type { CurrentQuotaStatus } from '../../app/types/subscriptions'

const { apiFetch, useApiFetchMock, useProfileQuotaResetsMock } = vi.hoisted(
  () => {
    const apiFetch = vi.fn()
    return {
      apiFetch,
      useApiFetchMock: vi.fn(() => apiFetch),
      useProfileQuotaResetsMock: vi.fn(),
    }
  },
)

mockNuxtImport('useApiFetch', () => useApiFetchMock)
mockNuxtImport('useProfileQuotaResets', () => useProfileQuotaResetsMock)

const underQuota: CurrentQuotaStatus = {
  hasSubscription: true,
  quota5hTokens: 1_000,
  quota7dTokens: 10_000,
  remaining5hTokens: 500,
  remaining7dTokens: 5_000,
  reset5hAt: '2026-10-24T15:00:00Z',
  reset7dAt: '2026-10-25T15:00:00Z',
  used5hTokens: 500,
  used7dTokens: 5_000,
}

const exhaustedQuota: CurrentQuotaStatus = {
  ...underQuota,
  remaining5hTokens: 0,
  remaining7dTokens: 0,
  used5hTokens: 1_000,
  used7dTokens: 12_000,
}

function createResetState(
  summaryValue: QuotaResetSummary | null = {
    available: 1,
    hasResettableQuota: true,
    maximum: 2,
    pendingReset: false,
  },
) {
  const summary = shallowRef(summaryValue)
  const error = shallowRef<string | null>(null)
  const loading = shallowRef(false)
  const using = shallowRef(false)
  const ensureLoaded = vi.fn(async () => summary.value)
  const useReset = vi.fn<() => Promise<UseQuotaResetResponse | null>>()

  return {
    ensureLoaded,
    error,
    loading,
    summary,
    useReset,
    using,
  }
}

function mountProbe() {
  const holder = {} as { state: ReturnType<typeof useQuotaExhaustionDialog> }
  const Probe = defineComponent({
    setup() {
      holder.state = useQuotaExhaustionDialog()
      return () => null
    },
  })
  const wrapper = mount(Probe)
  return { state: holder.state, wrapper }
}

describe('exhaustedQuotaWindows', () => {
  it('matches every finite window whose usage reached or exceeded its limit', () => {
    expect(exhaustedQuotaWindows(exhaustedQuota).map(({ key }) => key)).toEqual(
      ['5h', '7d'],
    )
    expect(
      exhaustedQuotaWindows({
        ...underQuota,
        quota5hTokens: 0,
        used5hTokens: 0,
      }).map(({ key }) => key),
    ).toEqual(['5h'])
  })

  it('returns only the exhausted window when the other remains available', () => {
    expect(
      exhaustedQuotaWindows({
        ...underQuota,
        remaining5hTokens: 0,
        used5hTokens: 1_000,
      }).map(({ key }) => key),
    ).toEqual(['5h'])
  })

  it('ignores unlimited, under-limit, and unsubscribed quotas', () => {
    expect(
      exhaustedQuotaWindows({
        ...underQuota,
        quota5hTokens: null,
        quota7dTokens: null,
      }),
    ).toEqual([])
    expect(exhaustedQuotaWindows(underQuota)).toEqual([])
    expect(
      exhaustedQuotaWindows({ ...exhaustedQuota, hasSubscription: false }),
    ).toEqual([])
  })
})

describe('useQuotaExhaustionDialog', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    useProfileQuotaResetsMock.mockReset()
  })

  it('opens after live quota confirms exhaustion and then loads reset state', async () => {
    const resets = createResetState()
    resets.error.value = 'Unable to load quota resets.'
    useProfileQuotaResetsMock.mockReturnValue(resets)
    apiFetch.mockResolvedValue(exhaustedQuota)

    const { state, wrapper } = mountProbe()
    await flushPromises()

    expect(apiFetch).toHaveBeenCalledOnce()
    expect(apiFetch).toHaveBeenCalledWith('/api/v1/me/quota')
    expect(resets.ensureLoaded).toHaveBeenCalledOnce()
    expect(state.windows.value.map(({ key }) => key)).toEqual(['5h', '7d'])
    expect(state.isOpen.value).toBe(true)
    expect(state.resetError.value).toBe('Unable to load quota resets.')
    wrapper.unmount()
  })

  it('does not load resets or open when live quota is unavailable or available', async () => {
    const resets = createResetState()
    useProfileQuotaResetsMock.mockReturnValue(resets)
    apiFetch.mockRejectedValueOnce(new Error('offline'))

    const first = mountProbe()
    await flushPromises()
    expect(first.state.isOpen.value).toBe(false)
    expect(resets.ensureLoaded).not.toHaveBeenCalled()
    first.wrapper.unmount()

    apiFetch.mockResolvedValueOnce(underQuota)
    const second = mountProbe()
    await flushPromises()
    expect(second.state.isOpen.value).toBe(false)
    expect(resets.ensureLoaded).not.toHaveBeenCalled()
    second.wrapper.unmount()
  })

  it('keeps dismissal for the current mount and reopens on a new mount', async () => {
    const resets = createResetState()
    useProfileQuotaResetsMock.mockReturnValue(resets)
    apiFetch.mockResolvedValue(exhaustedQuota)

    const first = mountProbe()
    await flushPromises()
    first.state.isOpen.value = false
    expect(first.state.isOpen.value).toBe(false)
    first.wrapper.unmount()

    const second = mountProbe()
    await flushPromises()
    expect(second.state.isOpen.value).toBe(true)
    second.wrapper.unmount()
  })

  it('applies authoritative quota returned by a completed reset', async () => {
    const resets = createResetState()
    resets.useReset.mockResolvedValue({
      quota: underQuota,
      status: 'applied',
      summary: {
        available: 0,
        hasResettableQuota: true,
        maximum: 2,
        pendingReset: false,
      },
    })
    useProfileQuotaResetsMock.mockReturnValue(resets)
    apiFetch.mockResolvedValue(exhaustedQuota)

    const { state, wrapper } = mountProbe()
    await flushPromises()
    await state.useReset()

    expect(resets.useReset).toHaveBeenCalledOnce()
    expect(state.isOpen.value).toBe(false)
    wrapper.unmount()
  })

  it('keeps the warning open when reset consumption fails', async () => {
    const resets = createResetState()
    resets.useReset.mockRejectedValue(new Error('reset failed'))
    useProfileQuotaResetsMock.mockReturnValue(resets)
    apiFetch.mockResolvedValue(exhaustedQuota)

    const { state, wrapper } = mountProbe()
    await flushPromises()
    const response = await state.useReset()

    expect(response).toBeNull()
    expect(state.isOpen.value).toBe(true)
    wrapper.unmount()
  })

  it('keeps pending reset visible, then reloads live quota when it resolves', async () => {
    const resets = createResetState()
    resets.useReset.mockImplementation(async () => {
      resets.summary.value = {
        available: 0,
        hasResettableQuota: true,
        maximum: 2,
        pendingReset: true,
      }
      return {
        status: 'pending',
        summary: resets.summary.value,
      }
    })
    useProfileQuotaResetsMock.mockReturnValue(resets)
    apiFetch
      .mockResolvedValueOnce(exhaustedQuota)
      .mockResolvedValueOnce(underQuota)

    const { state, wrapper } = mountProbe()
    await flushPromises()
    await state.useReset()

    expect(state.isOpen.value).toBe(true)
    expect(state.resetPending.value).toBe(true)

    resets.summary.value = {
      available: 0,
      hasResettableQuota: true,
      maximum: 2,
      pendingReset: false,
    }
    await flushPromises()

    expect(apiFetch).toHaveBeenCalledTimes(2)
    expect(state.isOpen.value).toBe(false)
    wrapper.unmount()
  })

  it('does not reopen a dismissed warning when a pending reset resolves', async () => {
    const resets = createResetState({
      available: 0,
      hasResettableQuota: true,
      maximum: 2,
      pendingReset: true,
    })
    useProfileQuotaResetsMock.mockReturnValue(resets)
    apiFetch.mockResolvedValue(exhaustedQuota)

    const { state, wrapper } = mountProbe()
    await flushPromises()
    state.isOpen.value = false

    resets.summary.value = {
      ...resets.summary.value!,
      pendingReset: false,
    }
    await flushPromises()

    expect(apiFetch).toHaveBeenCalledTimes(2)
    expect(state.windows.value).toHaveLength(2)
    expect(state.isOpen.value).toBe(false)
    wrapper.unmount()
  })
})
