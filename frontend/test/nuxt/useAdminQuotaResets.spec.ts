import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useAdminQuotaResetEvents } from '../../app/composables/useAdminQuotaResetEvents'
import { useAdminQuotaStates } from '../../app/composables/useAdminQuotaStates'
import type {
  AdminQuotaResetEventsResponse,
  AdminQuotaStatesResponse,
} from '../../app/types/quota-resets'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return { apiFetch, useApiFetchMock: vi.fn(() => apiFetch) }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

function quotaStatesResponse(name = 'Alice'): AdminQuotaStatesResponse {
  return {
    generatedAt: '2026-10-25T10:00:00Z',
    projectionSyncedAt: '2026-10-25T09:59:00Z',
    summary: {
      projectedAccounts: 4,
      atRiskAccounts: 2,
      exhaustedAccounts: 1,
    },
    items: [
      {
        accountId: 'user-1',
        accountType: 'user',
        displayName: name,
        planName: 'Builder',
        hasSubscription: true,
        status: 'warning',
        maximumPercent: 85,
        mostSaturatedWindow: '5h',
        nextResetAt: '2026-10-25T12:00:00Z',
        windows: [
          {
            window: '5h',
            usedTokens: 85,
            quotaTokens: 100,
            remainingTokens: 15,
            percent: 85,
            resetAt: '2026-10-25T12:00:00Z',
          },
          {
            window: '7d',
            usedTokens: 50,
            quotaTokens: 100,
            remainingTokens: 50,
            percent: 50,
            resetAt: '2026-10-29T12:00:00Z',
          },
        ],
        syncedAt: '2026-10-25T09:59:00Z',
      },
    ],
    page: 1,
    pageSize: 20,
    total: 1,
  }
}

function eventsResponse(): AdminQuotaResetEventsResponse {
  return {
    window: '30d',
    startsAt: '2026-09-25T00:00:00Z',
    endsAt: '2026-10-25T00:00:00Z',
    summary: {
      monthlyGrants: 3,
      adminGrants: 1,
      appliedResetUses: 2,
      pendingResetUsesTotal: 1,
    },
    items: [
      {
        id: 'event-1',
        target: {
          accountId: 'service-1',
          accountType: 'service',
          displayName: 'Build bot',
        },
        type: 'quota_reset_used',
        status: 'pending',
        delta: -1,
        balanceAfter: 0,
        actor: { type: 'user', userId: 'user-1', displayName: 'Alice' },
        grantMonth: null,
        resetCutoffAt: '2026-10-25T09:00:00Z',
        reconcileAttempts: 2,
        lastReconcileAttemptAt: '2026-10-25T09:30:00Z',
        appliedAt: null,
        createdAt: '2026-10-25T09:00:00Z',
      },
    ],
    page: 1,
    pageSize: 20,
    total: 1,
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve
  })
  return { promise, resolve }
}

function mountProbe<T>(factory: () => T) {
  const holder = {} as { state: T }
  const Probe = defineComponent({
    setup() {
      holder.state = factory()
      return () => null
    },
  })
  const wrapper = mount(Probe)
  return { state: holder.state, wrapper }
}

describe('admin quota reset composables', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('uses the documented defaults and keeps both list failures independent', async () => {
    apiFetch.mockImplementation((path: string) => {
      if (path.startsWith('/api/v1/admin/quota-resets/quota-states')) {
        return Promise.reject(new Error('projection unavailable'))
      }
      return Promise.resolve(eventsResponse())
    })

    const { state, wrapper } = mountProbe(() => ({
      events: useAdminQuotaResetEvents(),
      states: useAdminQuotaStates(),
    }))
    await flushPromises()

    expect(apiFetch).toHaveBeenCalledWith(
      '/api/v1/admin/quota-resets/quota-states?page=1&pageSize=20&sortBy=saturation&sortDir=desc&view=at-risk',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    expect(apiFetch).toHaveBeenCalledWith(
      '/api/v1/admin/quota-resets/events?page=1&pageSize=20&sortBy=createdAt&sortDir=desc&window=30d',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    expect(state.states.error.value).toContain('projection unavailable')
    expect(state.events.error.value).toBeNull()
    expect(state.events.items.value[0]?.target.displayName).toBe('Build bot')
    expect(state.events.summary.value.pendingResetUsesTotal).toBe(1)
    wrapper.unmount()
  })

  it('aborts and ignores stale quota-state responses', async () => {
    const first = deferred<AdminQuotaStatesResponse>()
    const second = deferred<AdminQuotaStatesResponse>()
    apiFetch
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)
    const { state, wrapper } = mountProbe(useAdminQuotaStates)
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledOnce())
    const firstSignal = apiFetch.mock.calls[0]?.[1]?.signal as AbortSignal

    state.setPage(2)
    await nextTick()

    expect(firstSignal.aborted).toBe(true)
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2))
    second.resolve({ ...quotaStatesResponse('New result'), page: 2 })
    await vi.waitFor(() =>
      expect(state.items.value[0]?.displayName).toBe('New result'),
    )

    first.resolve(quotaStatesResponse('Stale result'))
    await nextTick()
    expect(state.items.value[0]?.displayName).toBe('New result')
    wrapper.unmount()
  })

  it('builds the quota view, account, search, pagination, and sort filters', async () => {
    vi.useFakeTimers()
    apiFetch.mockResolvedValue(quotaStatesResponse())
    const { state, wrapper } = mountProbe(useAdminQuotaStates)
    await flushPromises()

    state.setPage(3)
    state.setView('all')
    state.setAccountType('service')
    state.setSort('resetAt', 'asc')
    state.setSearch('build bot')
    await vi.advanceTimersByTimeAsync(180)
    await nextTick()

    expect(state.page.value).toBe(1)
    expect(apiFetch).toHaveBeenLastCalledWith(
      '/api/v1/admin/quota-resets/quota-states?page=1&pageSize=20&sortBy=resetAt&sortDir=asc&view=all&accountType=service&search=build+bot',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    wrapper.unmount()
  })

  it('builds independent event period, type, status, search, and sort filters', async () => {
    vi.useFakeTimers()
    apiFetch.mockResolvedValue(eventsResponse())
    const { state, wrapper } = mountProbe(useAdminQuotaResetEvents)
    await flushPromises()

    state.setPage(4)
    state.setWindow('7d')
    state.setType('quota_reset_used')
    state.setStatus('pending')
    state.setSort('appliedAt', 'asc')
    state.setSearch('build bot')
    await vi.advanceTimersByTimeAsync(180)
    await nextTick()

    expect(state.page.value).toBe(1)
    expect(apiFetch).toHaveBeenLastCalledWith(
      '/api/v1/admin/quota-resets/events?page=1&pageSize=20&sortBy=appliedAt&sortDir=asc&window=7d&type=quota_reset_used&status=pending&search=build+bot',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    wrapper.unmount()
  })

  it('aborts and ignores stale reset-event responses independently', async () => {
    const first = deferred<AdminQuotaResetEventsResponse>()
    const second = deferred<AdminQuotaResetEventsResponse>()
    apiFetch
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)
    const { state, wrapper } = mountProbe(useAdminQuotaResetEvents)
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledOnce())
    const firstSignal = apiFetch.mock.calls[0]?.[1]?.signal as AbortSignal

    state.setWindow('7d')
    await nextTick()

    expect(firstSignal.aborted).toBe(true)
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2))
    second.resolve({
      ...eventsResponse(),
      window: '7d',
      items: [
        {
          ...eventsResponse().items[0]!,
          id: 'new-event',
        },
      ],
    })
    await vi.waitFor(() => expect(state.items.value[0]?.id).toBe('new-event'))

    first.resolve(eventsResponse())
    await nextTick()
    expect(state.items.value[0]?.id).toBe('new-event')
    wrapper.unmount()
  })
})
