import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DashboardPeriodKpis from '../../app/components/Dashboard/DashboardPeriodKpis.vue'
import type { DashboardCockpitOverviewResponse } from '../../app/types/user-service'

function overview(
  window: '7d' | 'all',
  withCost: boolean,
): DashboardCockpitOverviewResponse {
  const totals = {
    requests: 2,
    toolCalls: 1,
    inputTokens: 20,
    outputTokens: 0,
    cacheReadInputTokens: 0,
    cacheWriteInputTokens: 0,
    completionInputTokens: 20,
    completionOutputTokens: 0,
    completionTokens: 20,
    embeddingTokens: 0,
    totalTokens: 20,
    ...(withCost
      ? {
          estimatedCost: {
            inputUsd: 0,
            outputUsd: 0,
            embeddingUsd: 0,
            totalUsd: 0,
            rates: {
              inputUsdPer1MTokens: 0,
              outputUsdPer1MTokens: 0,
              embeddingUsdPer1MTokens: 0,
            },
          },
        }
      : {}),
  }

  return {
    window,
    startsAt: '2026-10-15T00:00:00Z',
    endsAt: '2026-10-22T00:00:00Z',
    totals,
    totalDurationMs: 200,
    averageDurationMs: 100,
    daily: [],
    costCoverage: { status: withCost ? 'complete' : 'unavailable' },
    previous: {
      startsAt: '2026-10-08T00:00:00Z',
      endsAt: '2026-10-15T00:00:00Z',
      totals: { ...totals, requests: 1, totalTokens: 10 },
      totalDurationMs: 100,
      averageDurationMs: 100,
      daily: [],
      costCoverage: { status: withCost ? 'complete' : 'unavailable' },
    },
  }
}

function mountKpis(data: DashboardCockpitOverviewResponse) {
  return mount(DashboardPeriodKpis, {
    props: { overview: data },
    global: {
      stubs: {
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: true,
        VIcon: true,
        VTooltip: {
          template: '<span><slot name="activator" :props="{}" /></span>',
        },
      },
    },
  })
}

describe('DashboardPeriodKpis', () => {
  it('distinguishes unavailable cost from a real zero estimate', () => {
    const unavailable = mountKpis(overview('7d', false))
    expect(unavailable.text()).toContain('Current-price estimate unavailable')
    expect(unavailable.text()).toContain('—')

    const zero = mountKpis(overview('7d', true))
    expect(zero.text()).toContain('$0.00')
    expect(zero.text()).toContain('Estimated at current prices')
  })

  it('never displays comparison deltas for all time', () => {
    const wrapper = mountKpis(overview('all', true))

    expect(wrapper.text()).not.toContain('vs previous period')
    expect(wrapper.text()).not.toContain('preceding period')
  })

  it('hides the cost delta unless both periods have complete coverage', () => {
    const complete = overview('7d', true)
    const completeCostCard = mountKpis(complete)
      .findAll('article')
      .find((card) => card.text().includes('Estimated cost'))

    expect(completeCostCard?.text()).toContain('No change vs previous period')

    const partialCurrent = overview('7d', true)
    partialCurrent.costCoverage = { status: 'partial' }
    const partialCurrentCostCard = mountKpis(partialCurrent)
      .findAll('article')
      .find((card) => card.text().includes('Estimated cost'))

    expect(partialCurrentCostCard?.text()).not.toContain('vs previous period')

    const partialPrevious = overview('7d', true)
    partialPrevious.previous!.costCoverage = { status: 'partial' }
    const partialPreviousCostCard = mountKpis(partialPrevious)
      .findAll('article')
      .find((card) => card.text().includes('Estimated cost'))

    expect(partialPreviousCostCard?.text()).not.toContain('vs previous period')
  })

  it('marks the duration help control with its minimum target class', () => {
    const wrapper = mountKpis(overview('7d', true))
    const help = wrapper.get(
      '[aria-label="About average completed request duration"]',
    )

    expect(help.classes()).toContain('dashboard-period-kpis__help')
  })

  it('supports a focused metric contract without changing personal defaults', () => {
    const focused = mount(DashboardPeriodKpis, {
      props: {
        overview: overview('7d', true),
        visibleMetrics: [
          'requests',
          'tokens',
          'cost',
          'tokens-per-request',
          'latency',
        ],
      },
      global: {
        stubs: {
          VAvatar: { template: '<span><slot /></span>' },
          VBtn: true,
          VIcon: true,
          VTooltip: {
            template: '<span><slot name="activator" :props="{}" /></span>',
          },
        },
      },
    })

    expect(focused.text()).not.toContain('Tool calls')
    expect(focused.text()).toContain('Tokens per request')
    expect(mountKpis(overview('7d', true)).text()).toContain('Tool calls')
  })
})
