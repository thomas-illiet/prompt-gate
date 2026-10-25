import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount, shallowMount } from '@vue/test-utils'
import { shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminQuotaResetsDashboard from '../../app/components/AdminQuotaResets/AdminQuotaResetsDashboard.vue'
import AdminQuotaResetEventsSection from '../../app/components/AdminQuotaResets/AdminQuotaResetEventsSection.vue'
import AdminQuotaResetKpis from '../../app/components/AdminQuotaResets/AdminQuotaResetKpis.vue'
import AdminQuotaStatesSection from '../../app/components/AdminQuotaResets/AdminQuotaStatesSection.vue'

const { useAdminQuotaResetEventsMock, useAdminQuotaStatesMock } = vi.hoisted(
  () => ({
    useAdminQuotaResetEventsMock: vi.fn(),
    useAdminQuotaStatesMock: vi.fn(),
  }),
)

mockNuxtImport('useAdminQuotaResetEvents', () => useAdminQuotaResetEventsMock)
mockNuxtImport('useAdminQuotaStates', () => useAdminQuotaStatesMock)

function quotaStatesState() {
  return {
    accountType: shallowRef('all'),
    error: shallowRef<string | null>('Projection unavailable'),
    generatedAt: shallowRef(null),
    items: shallowRef([]),
    loading: shallowRef(false),
    page: shallowRef(1),
    pageSize: shallowRef(20),
    projectionSyncedAt: shallowRef(null),
    reload: vi.fn(async () => undefined),
    search: shallowRef(''),
    setAccountType: vi.fn(),
    setPage: vi.fn(),
    setPageSize: vi.fn(),
    setSearch: vi.fn(),
    setSort: vi.fn(),
    setView: vi.fn(),
    sortBy: shallowRef('saturation'),
    sortDir: shallowRef('desc'),
    summary: shallowRef({
      projectedAccounts: 0,
      atRiskAccounts: 0,
      exhaustedAccounts: 0,
    }),
    total: shallowRef(0),
    view: shallowRef('at-risk'),
  }
}

function resetEventsState() {
  return {
    endsAt: shallowRef('2026-10-25T00:00:00Z'),
    error: shallowRef<string | null>(null),
    items: shallowRef([
      {
        id: 'event-1',
        target: {
          accountId: 'service-1',
          accountType: 'service',
          displayName: 'Build bot',
        },
      },
    ]),
    loading: shallowRef(false),
    page: shallowRef(1),
    pageSize: shallowRef(20),
    reload: vi.fn(async () => undefined),
    search: shallowRef(''),
    setPage: vi.fn(),
    setPageSize: vi.fn(),
    setSearch: vi.fn(),
    setSort: vi.fn(),
    setStatus: vi.fn(),
    setType: vi.fn(),
    setWindow: vi.fn(),
    sortBy: shallowRef('createdAt'),
    sortDir: shallowRef('desc'),
    startsAt: shallowRef('2026-09-25T00:00:00Z'),
    status: shallowRef('all'),
    summary: shallowRef({
      monthlyGrants: 1,
      adminGrants: 1,
      appliedResetUses: 1,
      pendingResetUsesTotal: 1,
    }),
    total: shallowRef(1),
    type: shallowRef('all'),
    window: shallowRef('30d'),
  }
}

describe('AdminQuotaResetsDashboard', () => {
  beforeEach(() => {
    useAdminQuotaStatesMock.mockReset()
    useAdminQuotaResetEventsMock.mockReset()
  })

  it('refreshes both independent lists and preserves the healthy list on peer error', async () => {
    const states = quotaStatesState()
    const events = resetEventsState()
    useAdminQuotaStatesMock.mockReturnValue(states)
    useAdminQuotaResetEventsMock.mockReturnValue(events)

    const wrapper = mount(AdminQuotaResetsDashboard, {
      global: {
        stubs: {
          AdminQuotaResetEventsSection: {
            props: ['error', 'items', 'loading', 'window'],
            template:
              '<section data-test="events" :data-error="error || \'\'" :data-items="items.length" :data-loading="loading" :data-window="window" />',
          },
          AdminQuotaResetKpis: true,
          AdminQuotaStatesSection: {
            props: ['error', 'items', 'loading', 'view'],
            template:
              '<section data-test="states" :data-error="error || \'\'" :data-items="items.length" :data-loading="loading" :data-view="view" />',
          },
          AppPageHeader: {
            template:
              '<header><slot name="meta" /><slot name="footer" /></header>',
          },
          VBtn: {
            inheritAttrs: false,
            emits: ['click'],
            template:
              '<button v-bind="$attrs" @click="$emit(\'click\')"><slot /></button>',
          },
          VChip: { template: '<span><slot /></span>' },
          VContainer: { template: '<main><slot /></main>' },
        },
      },
    })

    expect(wrapper.get('[data-test="states"]').attributes()).toMatchObject({
      'data-error': 'Projection unavailable',
      'data-items': '0',
      'data-view': 'at-risk',
    })
    expect(wrapper.get('[data-test="events"]').attributes()).toMatchObject({
      'data-error': '',
      'data-items': '1',
      'data-window': '30d',
    })

    await wrapper
      .get('[aria-label="Refresh quota projections and reset events"]')
      .trigger('click')
    expect(states.reload).toHaveBeenCalledOnce()
    expect(events.reload).toHaveBeenCalledOnce()
  })

  it('resolves all dashboard sections as imported Vue components', () => {
    useAdminQuotaStatesMock.mockReturnValue(quotaStatesState())
    useAdminQuotaResetEventsMock.mockReturnValue(resetEventsState())

    const wrapper = shallowMount(AdminQuotaResetsDashboard, {
      global: {
        stubs: {
          AppPageHeader: true,
          VContainer: { template: '<main><slot /></main>' },
        },
      },
    })

    expect(wrapper.findComponent(AdminQuotaResetKpis).exists()).toBe(true)
    expect(wrapper.findComponent(AdminQuotaStatesSection).exists()).toBe(true)
    expect(wrapper.findComponent(AdminQuotaResetEventsSection).exists()).toBe(
      true,
    )
  })
})
