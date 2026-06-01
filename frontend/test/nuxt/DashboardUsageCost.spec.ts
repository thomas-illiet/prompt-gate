import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardActivityChart from '../../app/components/Dashboard/DashboardActivityChart.vue'
import DashboardTokensKpi from '../../app/components/Dashboard/DashboardTokensKpi.vue'
import type {
  DashboardActivityResponse,
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

describe('dashboard usage cost display', () => {
  beforeEach(() => {
    reload.mockClear()
    useDashboardWidgetMock.mockClear()
    widgetState.data.value = null
    widgetState.error.value = null
    widgetState.loading.value = false
  })

  it('shows estimated cost on the tokens KPI when present', () => {
    widgetState.data.value = tokensResponse(estimatedCost(0.01, 0.1, 0.02))

    const wrapper = mount(DashboardTokensKpi, {
      props: { scope: 'self', window: '7d' },
      global: {
        stubs: {
          DashboardKpiCard: {
            props: ['caption', 'formatter', 'value'],
            template:
              '<section data-test="kpi" :data-caption="caption">{{ formatter(value) }}</section>',
          },
        },
      },
    })

    const caption = wrapper.get('[data-test="kpi"]').attributes('data-caption')
    expect(caption).toContain('$0.13 estimated')
    expect(caption).toContain('$0.01 in')
    expect(caption).toContain('$0.10 out')
    expect(caption).toContain('$0.02 emb')
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
              '<section data-test="kpi" :data-caption="caption">{{ formatter(value) }}</section>',
          },
        },
      },
    })

    const caption = wrapper.get('[data-test="kpi"]').attributes('data-caption')
    expect(caption).toBe('30 completion / 5 embedding')
  })

  it('shows estimated cost on the activity summary when present', () => {
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

    expect(wrapper.get('[data-test="actions"]').text()).toContain(
      '$0.13 estimated',
    )
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
})
