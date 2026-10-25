import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardCockpit from '../../app/components/Dashboard/DashboardCockpit.vue'
import DashboardPageHeader from '../../app/components/Dashboard/DashboardPageHeader.vue'
import DashboardPage from '../../app/pages/dashboard.vue'
import type { DashboardScope } from '../../app/types/user-service'

const {
  authUser,
  overviewReloadMock,
  overviewState,
  routeQuery,
  useAuthStoreMock,
  useDashboardOverviewMock,
} = vi.hoisted(() => {
  const authUser = { value: null as null | { id: string; role: string } }
  const overviewReloadMock = vi.fn()
  const routeQuery: Record<string, string> = {}
  const overviewState = {
    data: { value: null as null | Record<string, unknown> },
    error: { value: null as string | null },
    loading: { value: false },
    staleWarning: { value: null as string | null },
    updatedAt: { value: '2026-10-22T10:00:00Z' as string | null },
  }

  return {
    authUser,
    overviewReloadMock,
    overviewState,
    routeQuery,
    useAuthStoreMock: vi.fn(() => ({
      initialize: vi.fn(async () => undefined),
      isAuthenticated: true,
      isReady: true,
      waitUntilReady: vi.fn(async () => undefined),
      get user() {
        return authUser.value
      },
    })),
    useDashboardOverviewMock: vi.fn(() => ({
      ...overviewState,
      reload: overviewReloadMock,
    })),
  }
})

mockNuxtImport('useAuthStore', () => useAuthStoreMock)
mockNuxtImport('useDashboardOverview', () => useDashboardOverviewMock)
mockNuxtImport('useRoute', () => () => ({
  fullPath: '/dashboard',
  query: routeQuery,
}))

function overviewData(requests = 4) {
  return {
    window: '7d',
    startsAt: '2026-10-15T00:00:00Z',
    endsAt: '2026-10-22T00:00:00Z',
    totals: {
      requests,
      toolCalls: requests ? 1 : 0,
      inputTokens: 10,
      outputTokens: 10,
      cacheReadInputTokens: 0,
      cacheWriteInputTokens: 0,
      completionInputTokens: 10,
      completionOutputTokens: 10,
      completionTokens: 20,
      embeddingTokens: 0,
      totalTokens: requests ? 20 : 0,
    },
    totalDurationMs: requests ? 500 : 0,
    averageDurationMs: requests ? 125 : null,
    daily: [],
    costCoverage: { status: 'unavailable' },
  }
}

function mountCockpit(scope: DashboardScope = 'self') {
  return mount(DashboardCockpit, {
    props: { scope },
    global: {
      stubs: {
        AppEmptyState: {
          template: '<div data-test="empty"><slot name="actions" /></div>',
        },
        AppSectionCard: {
          template: '<section><slot /><slot name="actions" /></section>',
        },
        DashboardAnnualActivity: {
          props: ['refreshVersion'],
          template: '<div data-test="annual" :data-refresh="refreshVersion" />',
        },
        DashboardBreakdownPanel: {
          props: ['modelValue', 'scope', 'window', 'refreshVersion'],
          emits: ['update:modelValue'],
          template:
            '<div data-test="breakdown" :data-scope="scope" :data-window="window" :data-tab="modelValue" />',
        },
        DashboardCurrentState: {
          props: ['scope', 'refreshVersion'],
          template:
            '<div data-test="current-state" :data-scope="scope" :data-refresh="refreshVersion" />',
        },
        DashboardPeriodKpis: { template: '<div data-test="kpis" />' },
        DashboardTimeRangeSelect: {
          props: ['modelValue'],
          template: '<div data-test="window-select">{{ modelValue }}</div>',
        },
        DashboardTopTools: {
          props: ['scope', 'window', 'refreshVersion'],
          template:
            '<div data-test="top-tools" :data-scope="scope" :data-window="window" :data-refresh="refreshVersion" />',
        },
        DashboardTrend: { template: '<div data-test="trend" />' },
        VAlert: { template: '<div><slot /></div>' },
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: {
          props: ['to'],
          emits: ['click'],
          template:
            '<button data-test="button" :data-to="to || \'\'" @click="$emit(\'click\')"><slot /></button>',
        },
        VCard: {
          props: ['tag'],
          template: '<component :is="tag || \'div\'"><slot /></component>',
        },
        VContainer: { template: '<div><slot /></div>' },
        VIcon: true,
      },
    },
  })
}

describe('DashboardPage', () => {
  beforeEach(async () => {
    authUser.value = { id: 'user-1', role: 'user' }
    for (const key of Object.keys(routeQuery))
      Reflect.deleteProperty(routeQuery, key)
    await navigateTo('/dashboard')
    overviewState.data.value = overviewData()
    overviewState.error.value = null
    overviewState.loading.value = false
    overviewState.staleWarning.value = null
    overviewReloadMock.mockClear()
    useAuthStoreMock.mockClear()
    useDashboardOverviewMock.mockClear()
  })

  it('composes the personal dashboard route with a fixed self scope', () => {
    const wrapper = mount(DashboardPage, {
      global: {
        stubs: {
          DashboardCockpit: {
            props: ['scope'],
            template: '<div data-test="cockpit" :data-scope="scope" />',
          },
        },
      },
    })

    expect(wrapper.get('[data-test="cockpit"]').attributes('data-scope')).toBe(
      'self',
    )
  })

  it('keeps an administrator on My usage and removes the legacy global scope', async () => {
    authUser.value = { id: 'admin-1', role: 'admin' }
    routeQuery.scope = 'global'
    const router = useRouter()
    const replace = vi.spyOn(router, 'replace')

    const wrapper = mountCockpit('self')
    const header = wrapper.getComponent(DashboardPageHeader)

    expect(wrapper.get('h1').text()).toBe('My usage')
    expect(header.props()).toMatchObject({ window: '7d' })
    expect(wrapper.find('[data-test="scope-select"]').exists()).toBe(false)
    expect(
      wrapper.get('[data-test="current-state"]').attributes('data-scope'),
    ).toBe('self')
    expect(wrapper.find('[data-test="annual"]').exists()).toBe(true)

    await vi.waitFor(() => expect(replace).toHaveBeenCalled())
    const lastNavigation = replace.mock.calls.at(-1)?.[0]
    expect(lastNavigation).toEqual({
      query: {
        breakdown: 'models',
        metric: 'requests',
        window: '7d',
      },
    })
    replace.mockRestore()
  })

  it('renders the global cockpit with identities and no personal annual activity', () => {
    authUser.value = { id: 'admin-1', role: 'admin' }
    routeQuery.breakdown = 'identities'

    const wrapper = mountCockpit('global')

    expect(wrapper.get('h1').text()).toBe('Organization overview')
    expect(
      wrapper.get('[data-test="current-state"]').attributes('data-scope'),
    ).toBe('global')
    expect(wrapper.get('[data-test="breakdown"]').attributes('data-tab')).toBe(
      'identities',
    )
    expect(wrapper.find('[data-test="annual"]').exists()).toBe(false)
  })

  it('hydrates window, metric, and breakdown from the URL', () => {
    routeQuery.window = '30d'
    routeQuery.metric = 'tokens'
    routeQuery.breakdown = 'providers'

    const wrapper = mountCockpit()

    expect(wrapper.get('[data-test="window-select"]').text()).toBe('30d')
    expect(wrapper.get('[data-test="breakdown"]').attributes('data-tab')).toBe(
      'providers',
    )
    expect(
      wrapper.get('[data-test="top-tools"]').attributes('data-window'),
    ).toBe('30d')
    expect(useDashboardOverviewMock).toHaveBeenCalled()
  })

  it('refreshes the overview and every lazy dashboard area together', async () => {
    const wrapper = mountCockpit()
    const refresh = wrapper
      .findAll('[data-test="button"]')
      .find((button) => button.text().includes('Refresh'))
    expect(refresh).toBeDefined()

    await refresh!.trigger('click')

    expect(overviewReloadMock).toHaveBeenCalledTimes(1)
    expect(
      wrapper.get('[data-test="current-state"]').attributes('data-refresh'),
    ).toBe('1')
    expect(
      wrapper.get('[data-test="top-tools"]').attributes('data-refresh'),
    ).toBe('1')
    expect(wrapper.get('[data-test="annual"]').attributes('data-refresh')).toBe(
      '1',
    )
  })

  it('sends limited users to the accessible FAQ from the single empty state', () => {
    authUser.value = { id: 'limited-1', role: 'limited' }
    overviewState.data.value = overviewData(0)

    const wrapper = mountCockpit()

    const action = wrapper
      .findAll('[data-test="button"]')
      .find((button) => button.text().includes('Browse FAQ'))
    expect(action?.attributes('data-to')).toBe('/faq')
    expect(wrapper.findAll('[data-test="empty"]')).toHaveLength(1)
  })
})
