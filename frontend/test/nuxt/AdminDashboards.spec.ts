import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount, shallowMount } from '@vue/test-utils'
import { defineComponent, h, shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminActivityDashboard from '../../app/components/AdminDashboards/AdminActivityDashboard.vue'
import AdminDashboardOverview from '../../app/components/AdminDashboards/AdminDashboardOverview.vue'
import AdminDashboardThemeCard from '../../app/components/AdminDashboards/AdminDashboardThemeCard.vue'
import AdminUsageDashboard from '../../app/components/AdminDashboards/AdminUsageDashboard.vue'
import type { DashboardCockpitOverviewResponse } from '../../app/types/user-service'

const {
  overviewReload,
  quotaReload,
  useAdminDashboardAnalysisQueryMock,
  useAdminDashboardQuotaRiskMock,
  useAdminDashboardWindowQueryMock,
  useDashboardOverviewMock,
} = vi.hoisted(() => ({
  overviewReload: vi.fn(),
  quotaReload: vi.fn(),
  useAdminDashboardAnalysisQueryMock: vi.fn(),
  useAdminDashboardQuotaRiskMock: vi.fn(),
  useAdminDashboardWindowQueryMock: vi.fn(),
  useDashboardOverviewMock: vi.fn(),
}))

mockNuxtImport(
  'useAdminDashboardAnalysisQuery',
  () => useAdminDashboardAnalysisQueryMock,
)
mockNuxtImport(
  'useAdminDashboardQuotaRisk',
  () => useAdminDashboardQuotaRiskMock,
)
mockNuxtImport(
  'useAdminDashboardWindowQuery',
  () => useAdminDashboardWindowQueryMock,
)
mockNuxtImport('useDashboardOverview', () => useDashboardOverviewMock)

const HeaderStub = defineComponent({
  props: ['title', 'window'],
  emits: ['refresh', 'update:window'],
  template:
    '<header data-test="header" :data-title="title" :data-window="window"><button data-test="refresh" @click="$emit(\'refresh\')">Refresh</button></header>',
})

const ThemeCardStub = defineComponent({
  props: ['title', 'to', 'stats'],
  setup(props) {
    return () =>
      h('article', {
        'data-test': 'theme-card',
        'data-title': props.title,
        'data-to':
          typeof props.to === 'string'
            ? props.to
            : `${props.to.path}?window=${props.to.query.window}`,
      })
  },
})

function overviewData(): DashboardCockpitOverviewResponse {
  return {
    window: '7d',
    startsAt: '2026-10-18T00:00:00Z',
    endsAt: '2026-10-25T00:00:00Z',
    totals: {
      requests: 10,
      toolCalls: 4,
      inputTokens: 100,
      outputTokens: 50,
      cacheReadInputTokens: 0,
      cacheWriteInputTokens: 0,
      completionInputTokens: 100,
      completionOutputTokens: 50,
      completionTokens: 150,
      embeddingTokens: 0,
      totalTokens: 150,
    },
    totalDurationMs: 500,
    averageDurationMs: 50,
    daily: [],
    costCoverage: { status: 'unavailable' },
    adoption: {
      activeUsers: 3,
      activeServiceAccounts: 2,
      activeVirtualKeys: 4,
    },
  }
}

function commonStubs() {
  return {
    AppSectionCard: {
      template: '<section><slot name="actions" /><slot /></section>',
    },
    DashboardPageHeader: HeaderStub,
    VAlert: { template: '<div><slot /></div>' },
    VAvatar: { template: '<span><slot /></span>' },
    VBtn: { template: '<button><slot /></button>' },
    VContainer: { template: '<main><slot /></main>' },
    VIcon: true,
  }
}

describe('thematic admin dashboard containers', () => {
  beforeEach(() => {
    overviewReload.mockReset()
    quotaReload.mockReset()
    useAdminDashboardAnalysisQueryMock.mockReset()
    useAdminDashboardQuotaRiskMock.mockReset()
    useAdminDashboardWindowQueryMock.mockReset()
    useDashboardOverviewMock.mockReset()

    useAdminDashboardWindowQueryMock.mockReturnValue(shallowRef('7d'))
    useAdminDashboardAnalysisQueryMock.mockImplementation(
      (allowed: string[]) => ({
        breakdown: shallowRef(
          allowed.includes('identities') ? 'identities' : 'models',
        ),
        metric: shallowRef('requests'),
        window: shallowRef('7d'),
      }),
    )
    useDashboardOverviewMock.mockReturnValue({
      data: shallowRef(overviewData()),
      error: shallowRef(null),
      loading: shallowRef(false),
      reload: overviewReload,
      staleWarning: shallowRef(null),
      updatedAt: shallowRef('2026-10-25T10:00:00Z'),
    })
    useAdminDashboardQuotaRiskMock.mockReturnValue({
      data: shallowRef({
        atRiskCount: 2,
        exhaustedCount: 1,
        items: [],
      }),
      error: shallowRef(null),
      loading: shallowRef(false),
      reload: quotaReload,
    })
  })

  it('keeps Overview compact with exactly three linked summary cards', async () => {
    const wrapper = mount(AdminDashboardOverview, {
      global: {
        stubs: {
          ...commonStubs(),
          AdminDashboardThemeCard: ThemeCardStub,
        },
      },
    })

    const cards = wrapper.findAll('[data-test="theme-card"]')
    expect(cards.map((card) => card.attributes('data-title'))).toEqual([
      'Usage & costs',
      'Activity & adoption',
      'Quotas & resets',
    ])
    expect(cards.map((card) => card.attributes('data-to'))).toEqual([
      '/admin/usage?window=7d',
      '/admin/activity?window=7d',
      '/admin/quota-resets',
    ])
    expect(wrapper.findComponent({ name: 'DashboardTrend' }).exists()).toBe(
      false,
    )
    expect(wrapper.find('table').exists()).toBe(false)

    await wrapper.get('[data-test="refresh"]').trigger('click')
    expect(overviewReload).toHaveBeenCalledOnce()
  })

  it('resolves the three overview cards as imported Vue components', () => {
    const wrapper = shallowMount(AdminDashboardOverview, {
      global: { stubs: commonStubs() },
    })

    expect(wrapper.findAllComponents(AdminDashboardThemeCard)).toHaveLength(3)
  })

  it('places only usage KPIs, trend, and infrastructure breakdown on Usage', async () => {
    const wrapper = mount(AdminUsageDashboard, {
      global: {
        stubs: {
          ...commonStubs(),
          DashboardBreakdownPanel: {
            props: [
              'allowedBreakdowns',
              'metric',
              'modelValue',
              'refreshVersion',
            ],
            template:
              '<div data-test="breakdown" :data-allowed="allowedBreakdowns.join(\',\')" :data-refresh="refreshVersion" />',
          },
          DashboardPeriodKpis: {
            props: ['visibleMetrics'],
            template:
              '<div data-test="period-kpis" :data-metrics="visibleMetrics.join(\',\')" />',
          },
          DashboardTrend: { template: '<div data-test="trend" />' },
        },
      },
    })

    expect(
      wrapper.get('[data-test="period-kpis"]').attributes('data-metrics'),
    ).toBe('requests,tokens,cost,tokens-per-request,latency')
    expect(
      wrapper.get('[data-test="breakdown"]').attributes('data-allowed'),
    ).toBe('models,providers,provider-types')
    expect(wrapper.get('[data-test="trend"]')).toBeDefined()
    expect(wrapper.findComponent({ name: 'DashboardTopTools' }).exists()).toBe(
      false,
    )

    await wrapper.get('[data-test="refresh"]').trigger('click')
    expect(overviewReload).toHaveBeenCalledOnce()
    expect(
      wrapper.get('[data-test="breakdown"]').attributes('data-refresh'),
    ).toBe('1')
  })

  it('places adoption, identity ranking, and top tools on Activity', async () => {
    const wrapper = mount(AdminActivityDashboard, {
      global: {
        stubs: {
          ...commonStubs(),
          DashboardBreakdownPanel: {
            props: ['allowedBreakdowns', 'refreshVersion'],
            template:
              '<div data-test="identity-ranking" :data-allowed="allowedBreakdowns.join(\',\')" :data-refresh="refreshVersion" />',
          },
          DashboardKpiCard: {
            props: ['title'],
            template: '<div data-test="adoption-kpi" :data-title="title" />',
          },
          DashboardMetricSelect: {
            template: '<div data-test="metric-select" />',
          },
          DashboardTopTools: {
            props: ['refreshVersion'],
            template:
              '<div data-test="top-tools" :data-refresh="refreshVersion" />',
          },
        },
      },
    })

    expect(
      wrapper
        .findAll('[data-test="adoption-kpi"]')
        .map((card) => card.attributes('data-title')),
    ).toEqual([
      'Active users',
      'Active service accounts',
      'Active virtual keys',
    ])
    expect(
      wrapper.get('[data-test="identity-ranking"]').attributes('data-allowed'),
    ).toBe('identities')
    expect(wrapper.get('[data-test="metric-select"]')).toBeDefined()
    expect(wrapper.get('[data-test="top-tools"]')).toBeDefined()
    expect(wrapper.findComponent({ name: 'DashboardTrend' }).exists()).toBe(
      false,
    )

    await wrapper.get('[data-test="refresh"]').trigger('click')
    expect(overviewReload).toHaveBeenCalledOnce()
    expect(
      wrapper.get('[data-test="identity-ranking"]').attributes('data-refresh'),
    ).toBe('1')
    expect(
      wrapper.get('[data-test="top-tools"]').attributes('data-refresh'),
    ).toBe('1')
  })
})
