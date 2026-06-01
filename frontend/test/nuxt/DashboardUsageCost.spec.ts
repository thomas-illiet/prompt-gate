import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { defineComponent, type Component } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardActivityChart from '../../app/components/Dashboard/DashboardActivityChart.vue'
import DashboardBreakdownBarChart from '../../app/components/Dashboard/DashboardBreakdownBarChart.vue'
import DashboardBreakdownPieChart from '../../app/components/Dashboard/DashboardBreakdownPieChart.vue'
import DashboardBreakdownWidget from '../../app/components/Dashboard/DashboardBreakdownWidget.vue'
import DashboardTokensKpi from '../../app/components/Dashboard/DashboardTokensKpi.vue'
import DashboardUsageLineChart from '../../app/components/Dashboard/DashboardUsageLineChart.vue'
import type {
  DashboardActivityResponse,
  DashboardBreakdownResponse,
  DashboardTokensResponse,
  EstimatedCost,
} from '../../app/types/user-service'

const { reload, useDashboardWidgetMock, widgetState } = vi.hoisted(() => {
  const reload = vi.fn()
  const widgetState = {
    data: { value: null as unknown },
    error: { value: null as string | null },
    loading: { value: false },
    reload,
  }

  return {
    reload,
    useDashboardWidgetMock: vi.fn(() => widgetState),
    widgetState,
  }
})

mockNuxtImport('useDashboardWidget', () => useDashboardWidgetMock)

const rates = {
  inputUsdPer1MTokens: 5,
  outputUsdPer1MTokens: 30,
  embeddingUsdPer1MTokens: 0.02,
}

function estimatedCost(
  inputUsd: number,
  outputUsd: number,
  embeddingUsd: number,
): EstimatedCost {
  return {
    inputUsd,
    outputUsd,
    embeddingUsd,
    totalUsd: inputUsd + outputUsd + embeddingUsd,
    rates,
  }
}

function tokensResponse(cost?: EstimatedCost): DashboardTokensResponse {
  return {
    window: '7d',
    startsAt: '2026-01-01T00:00:00Z',
    endsAt: '2026-01-07T00:00:00Z',
    inputTokens: 10,
    outputTokens: 20,
    cacheReadInputTokens: 0,
    cacheWriteInputTokens: 0,
    completionInputTokens: 10,
    completionOutputTokens: 20,
    completionTokens: 30,
    embeddingTokens: 5,
    totalTokens: 35,
    ...(cost ? { estimatedCost: cost } : {}),
  }
}

function activityResponse(cost?: EstimatedCost): DashboardActivityResponse {
  return {
    window: '7d',
    startsAt: '2026-01-01T00:00:00Z',
    endsAt: '2026-01-07T00:00:00Z',
    daily: [
      {
        date: '2026-01-01',
        requests: 2,
        prompts: 1,
        inputTokens: 10,
        outputTokens: 20,
        completionInputTokens: 10,
        completionOutputTokens: 20,
        completionTokens: 30,
        embeddingTokens: 5,
        totalTokens: 35,
        ...(cost ? { estimatedCost: cost } : {}),
      },
    ],
  }
}

function breakdownResponse(cost?: EstimatedCost): DashboardBreakdownResponse {
  return {
    window: '7d',
    startsAt: '2026-01-01T00:00:00Z',
    endsAt: '2026-01-07T00:00:00Z',
    items: [
      {
        name: 'gpt-5',
        requests: 2,
        totalTokens: 35,
        ...(cost ? { estimatedCost: cost } : {}),
      },
    ],
  }
}

interface ChartOptionForTest {
  series?: Array<{
    data?: unknown[]
    name?: string
  }>
  tooltip?: {
    appendTo?: string
    confine?: boolean
    extraCssText?: string
    formatter?: unknown
  }
}

const VChartStub = defineComponent({
  name: 'VChartStub',
  props: {
    autoresize: Boolean,
    option: {
      required: true,
      type: Object,
    },
  },
  template: '<div data-test="chart"></div>',
})

function mountChartOption(component: Component, props: Record<string, unknown>) {
  const wrapper = mount(component, {
    props,
    global: {
      stubs: {
        Echarts: VChartStub,
        'v-chart': VChartStub,
      },
    },
  })

  return wrapper.getComponent(VChartStub).props(
    'option',
  ) as ChartOptionForTest
}

function tooltipFormatter(option: ChartOptionForTest) {
  expect(option.tooltip?.appendTo).toBe('body')
  expect(option.tooltip?.confine).toBe(true)
  expect(option.tooltip?.extraCssText).toContain('max-width')
  expect(option.tooltip?.extraCssText).toContain('border-radius')
  expect(option.tooltip?.formatter).toBeTypeOf('function')
  return option.tooltip?.formatter as (params: unknown) => string
}

function seriesData(option: ChartOptionForTest, index = 0) {
  const data = option.series?.[index]?.data
  expect(data).toBeDefined()
  return data ?? []
}

function activityTooltipParams(option: ChartOptionForTest) {
  return (
    option.series?.map((series) => ({
      axisValueLabel: '2026-01-01',
      data: series.data?.[0],
      seriesName: series.name,
    })) ?? []
  )
}

describe('dashboard usage cost display', () => {
  beforeEach(() => {
    reload.mockClear()
    useDashboardWidgetMock.mockClear()
    widgetState.data.value = null
    widgetState.error.value = null
    widgetState.loading.value = false
  })

  it('keeps the tokens KPI cost-free when estimatedCost is present', () => {
    widgetState.data.value = tokensResponse(estimatedCost(0.01, 0.1, 0.02))

    const wrapper = mount(DashboardTokensKpi, {
      props: { scope: 'self', window: '7d' },
      global: {
        stubs: {
          DashboardKpiCard: {
            props: ['caption', 'formatter', 'value'],
            template:
              `<section data-test="kpi" :data-caption="caption || ''">{{ formatter(value) }}</section>`,
          },
        },
      },
    })

    const caption = wrapper.get('[data-test="kpi"]').attributes('data-caption')
    expect(caption).toBe('')
  })

  it('keeps the tokens KPI cost-free when estimatedCost is absent', () => {
    widgetState.data.value = tokensResponse()

    const wrapper = mount(DashboardTokensKpi, {
      props: { scope: 'self', window: '7d' },
      global: {
        stubs: {
          DashboardKpiCard: {
            props: ['caption', 'formatter', 'value'],
            template:
              `<section data-test="kpi" :data-caption="caption || ''">{{ formatter(value) }}</section>`,
          },
        },
      },
    })

    const caption = wrapper.get('[data-test="kpi"]').attributes('data-caption')
    expect(caption).toBe('')
  })

  it('keeps the activity summary cost-free when estimatedCost is present', () => {
    widgetState.data.value = activityResponse(estimatedCost(0.01, 0.1, 0.02))

    const wrapper = mount(DashboardActivityChart, {
      props: { scope: 'self', window: '7d' },
      global: {
        stubs: {
          AppSectionCard: {
            template:
              '<section><div data-test="actions"><slot name="actions" /></div><slot /></section>',
          },
          AppEmptyState: true,
          DashboardUsageLineChart: true,
          VAlert: true,
          VBtn: true,
          VChip: {
            template: '<span data-test="chip"><slot /></span>',
          },
        },
      },
    })

    const actions = wrapper.get('[data-test="actions"]').text()
    expect(actions).toContain('10 input')
    expect(actions).toContain('20 output')
    expect(actions).toContain('5 embedding')
    expect(actions).not.toContain('$')
    expect(actions).not.toContain('estimated')
  })

  it('keeps the activity summary cost-free when estimatedCost is absent', () => {
    widgetState.data.value = activityResponse()

    const wrapper = mount(DashboardActivityChart, {
      props: { scope: 'self', window: '7d' },
      global: {
        stubs: {
          AppSectionCard: {
            template:
              '<section><div data-test="actions"><slot name="actions" /></div><slot /></section>',
          },
          AppEmptyState: true,
          DashboardUsageLineChart: true,
          VAlert: true,
          VBtn: true,
          VChip: {
            template: '<span data-test="chip"><slot /></span>',
          },
        },
      },
    })

    expect(wrapper.get('[data-test="actions"]').text()).not.toContain('$')
  })

  it('keeps token breakdown leader cards cost-free when present', () => {
    widgetState.data.value = breakdownResponse(estimatedCost(0.01, 0.1, 0.02))

    const wrapper = mount(DashboardBreakdownWidget, {
      props: {
        endpoint: '/api/v1/me/dashboard/top-models',
        emptyLabel: 'No model usage yet',
        icon: 'mdi-cube-outline',
        subtitle: 'By token volume',
        title: 'Top models',
        window: '7d',
      },
      global: {
        stubs: {
          AppSectionCard: {
            template:
              '<section><div data-test="actions"><slot name="actions" /></div><slot /></section>',
          },
          AppEmptyState: true,
          DashboardBreakdownBarChart: true,
          DashboardBreakdownPieChart: true,
          VAlert: true,
          VBtn: true,
          VChip: {
            template: '<span data-test="chip"><slot /></span>',
          },
        },
      },
    })

    const actions = wrapper.get('[data-test="actions"]').text()
    expect(actions).toContain('gpt-5 / 35')
    expect(actions).not.toContain('$')
  })

  it('adds estimated cost metadata to breakdown bar tooltips when present', () => {
    const option = mountChartOption(DashboardBreakdownBarChart, {
      items: breakdownResponse(estimatedCost(0.01, 0.1, 0.02)).items,
    })

    const html = tooltipFormatter(option)([
      {
        data: seriesData(option)[0],
        name: 'gpt-5',
      },
    ])

    expect(html).toContain('gpt-5')
    expect(html).toContain('data-dashboard-tooltip')
    expect(html).toContain('35 tokens')
    expect(html).toContain('2 requests')
    expect(html).toContain('Estimated cost')
    expect(html).toContain('Total')
    expect(html).toContain('$0.13')
    expect(html).toContain('Input')
    expect(html).toContain('$0.01')
    expect(html).toContain('Output')
    expect(html).toContain('$0.10')
    expect(html).toContain('Embedding')
    expect(html).toContain('$0.02')
  })

  it('adds estimated cost metadata to breakdown pie tooltips when present', () => {
    const option = mountChartOption(DashboardBreakdownPieChart, {
      items: breakdownResponse(estimatedCost(0.01, 0.1, 0.02)).items,
    })

    const html = tooltipFormatter(option)({
      data: seriesData(option)[0],
      name: 'gpt-5',
    })

    expect(html).toContain('gpt-5')
    expect(html).toContain('data-dashboard-tooltip')
    expect(html).toContain('35 tokens')
    expect(html).toContain('2 requests')
    expect(html).toContain('Estimated cost')
    expect(html).toContain('Total')
    expect(html).toContain('$0.13')
    expect(html).toContain('Input')
    expect(html).toContain('$0.01')
    expect(html).toContain('Output')
    expect(html).toContain('$0.10')
    expect(html).toContain('Embedding')
    expect(html).toContain('$0.02')
  })

  it('adds estimated cost metadata to daily activity tooltips when present', () => {
    const option = mountChartOption(DashboardUsageLineChart, {
      daily: activityResponse(estimatedCost(0.01, 0.1, 0.02)).daily,
    })

    const html = tooltipFormatter(option)(activityTooltipParams(option))

    expect(html).toContain('2026-01-01')
    expect(html).toContain('data-dashboard-tooltip')
    expect(html).toContain('Requests')
    expect(html).toContain('2')
    expect(html).toContain('Input tokens')
    expect(html).toContain('10')
    expect(html).toContain('Output tokens')
    expect(html).toContain('20')
    expect(html).toContain('Embedding tokens')
    expect(html).toContain('5')
    expect(html).toContain('Estimated cost')
    expect(html).toContain('Total')
    expect(html).toContain('$0.13')
    expect(html).toContain('Input')
    expect(html).toContain('$0.01')
    expect(html).toContain('Output')
    expect(html).toContain('$0.10')
    expect(html).toContain('Embedding')
    expect(html).toContain('$0.02')
  })

  it('keeps chart tooltips cost-free when estimatedCost is absent', () => {
    const barOption = mountChartOption(DashboardBreakdownBarChart, {
      items: breakdownResponse().items,
    })
    const pieOption = mountChartOption(DashboardBreakdownPieChart, {
      items: breakdownResponse().items,
    })
    const activityOption = mountChartOption(DashboardUsageLineChart, {
      daily: activityResponse().daily,
    })

    const html = [
      tooltipFormatter(barOption)([
        { data: seriesData(barOption)[0], name: 'gpt-5' },
      ]),
      tooltipFormatter(pieOption)({
        data: seriesData(pieOption)[0],
        name: 'gpt-5',
      }),
      tooltipFormatter(activityOption)(activityTooltipParams(activityOption)),
    ].join('\n')

    expect(html).not.toContain('$')
    expect(html).not.toContain('Estimated cost')
  })
})
