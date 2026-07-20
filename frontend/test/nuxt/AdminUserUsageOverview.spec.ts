import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent } from 'vue'

import AdminUserUsageOverview from '../../app/components/AdminUsers/AdminUserUsageOverview.vue'
import type {
  DailyUsage,
  DashboardOverviewResponse,
} from '../../app/types/user-service'

const day: DailyUsage = {
  date: '2026-07-13',
  requests: 3,
  prompts: 2,
  inputTokens: 120,
  outputTokens: 30,
  completionInputTokens: 100,
  completionOutputTokens: 30,
  completionTokens: 130,
  embeddingTokens: 20,
  totalTokens: 150,
}

const response: DashboardOverviewResponse = {
  window: '7d',
  startsAt: '2026-07-07T00:00:00Z',
  endsAt: '2026-07-13T23:59:59Z',
  totals: {
    requests: 3,
    prompts: 2,
    toolCalls: 1,
    inputTokens: 120,
    outputTokens: 30,
    cacheReadInputTokens: 0,
    cacheWriteInputTokens: 0,
    completionInputTokens: 100,
    completionOutputTokens: 30,
    completionTokens: 130,
    embeddingTokens: 20,
    totalTokens: 150,
    estimatedCost: {
      inputUsd: 0.01,
      outputUsd: 0.02,
      embeddingUsd: 0.001,
      totalUsd: 0.031,
      rates: {
        inputUsdPer1MTokens: 1,
        outputUsdPer1MTokens: 2,
        embeddingUsdPer1MTokens: 0.1,
      },
    },
  },
  totalDurationMs: 4500,
  daily: [day],
}

const KpiStub = defineComponent({
  props: {
    caption: String,
    loading: Boolean,
    title: String,
    value: Number,
  },
  template: `
    <article
      data-test="kpi"
      :data-loading="loading"
      :data-title="title"
      :data-value="value == null ? '' : value"
    >
      {{ title }} {{ caption }}
    </article>
  `,
})

function mountOverview(options?: {
  data?: DashboardOverviewResponse | null
  error?: string | null
  loading?: boolean
}) {
  return mount(AdminUserUsageOverview, {
    props: {
      data: options?.data === undefined ? response : options.data,
      error: options?.error ?? null,
      loading: options?.loading ?? false,
    },
    global: {
      stubs: {
        AppEmptyState: {
          props: ['text', 'title'],
          template:
            '<section data-test="empty"><h4>{{ title }}</h4><p>{{ text }}</p></section>',
        },
        DashboardKpiCard: KpiStub,
        DashboardUsageLineChart: {
          props: ['daily'],
          template:
            '<div data-test="usage-chart" :data-days="daily.length" />',
        },
        VAlert: { template: '<aside><slot /></aside>' },
        VBtn: {
          emits: ['click'],
          template:
            '<button data-test="retry" @click="$emit(\'click\')"><slot /></button>',
        },
        VChip: { template: '<span data-test="activity-chip"><slot /></span>' },
      },
    },
  })
}

describe('AdminUserUsageOverview', () => {
  it('renders only the four compact usage KPIs and daily activity', () => {
    const wrapper = mountOverview()
    const kpis = wrapper.findAll('[data-test="kpi"]')

    expect(kpis).toHaveLength(4)
    expect(kpis.map((kpi) => kpi.attributes('data-title'))).toEqual([
      'Estimated cost',
      'Total tokens',
      'Requests',
      'Total duration',
    ])
    expect(kpis.map((kpi) => kpi.attributes('data-value'))).toEqual([
      '0.031',
      '150',
      '3',
      '4500',
    ])
    expect(wrapper.get('[data-test="usage-chart"]').attributes('data-days')).toBe(
      '1',
    )
    expect(wrapper.text()).not.toContain('Adoption')
    expect(wrapper.text()).not.toContain('Rankings')
    expect(wrapper.text()).not.toContain('Prompts')
  })

  it('shows loading placeholders before the first response', () => {
    const wrapper = mountOverview({ data: null, loading: true })

    expect(wrapper.findAll('[data-test="kpi"]')).toHaveLength(4)
    expect(
      wrapper
        .findAll('[data-test="kpi"]')
        .every((kpi) => kpi.attributes('data-loading') === 'true'),
    ).toBe(true)
    expect(wrapper.find('[aria-busy="true"]').exists()).toBe(true)
  })

  it('shows a retryable initial error without empty metric content', async () => {
    const wrapper = mountOverview({
      data: null,
      error: 'This user no longer exists.',
    })

    expect(wrapper.text()).toContain('This user no longer exists.')
    expect(wrapper.findAll('[data-test="kpi"]')).toHaveLength(0)

    await wrapper.get('[data-test="retry"]').trigger('click')

    expect(wrapper.emitted('retry')).toHaveLength(1)
  })

  it('recognizes zero-filled daily buckets as an empty usage window', () => {
    const emptyDay: DailyUsage = {
      ...day,
      requests: 0,
      prompts: 0,
      inputTokens: 0,
      outputTokens: 0,
      completionInputTokens: 0,
      completionOutputTokens: 0,
      completionTokens: 0,
      embeddingTokens: 0,
      totalTokens: 0,
    }
    const wrapper = mountOverview({
      data: {
        ...response,
        totals: {
          ...response.totals,
          requests: 0,
          prompts: 0,
          toolCalls: 0,
          inputTokens: 0,
          outputTokens: 0,
          completionInputTokens: 0,
          completionOutputTokens: 0,
          completionTokens: 0,
          embeddingTokens: 0,
          totalTokens: 0,
          estimatedCost: undefined,
        },
        totalDurationMs: 0,
        daily: Array.from({ length: 7 }, (_, index) => ({
          ...emptyDay,
          date: `2026-07-${String(index + 7).padStart(2, '0')}`,
        })),
      },
    })

    expect(wrapper.get('[data-test="empty"]').text()).toContain('No usage yet')
    expect(wrapper.find('[data-test="usage-chart"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="activity-chip"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-test="kpi"]')).toHaveLength(4)
  })
})
