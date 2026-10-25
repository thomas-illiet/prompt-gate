import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardTrend from '../../app/components/Dashboard/DashboardTrend.vue'
import type { DailyUsage } from '../../app/types/user-service'

const { downloadDailyUsageCsvMock } = vi.hoisted(() => ({
  downloadDailyUsageCsvMock: vi.fn(),
}))

vi.mock('../../app/utils/daily-usage-csv', () => ({
  downloadDailyUsageCsv: downloadDailyUsageCsvMock,
}))

const daily: DailyUsage[] = [
  {
    date: '2026-10-23',
    requests: 4,
    inputTokens: 800,
    outputTokens: 200,
    completionInputTokens: 800,
    completionOutputTokens: 200,
    completionTokens: 1_000,
    embeddingTokens: 0,
    totalTokens: 1_000,
  },
  {
    date: '2026-10-24',
    requests: 6,
    inputTokens: 1_200,
    outputTokens: 300,
    completionInputTokens: 1_200,
    completionOutputTokens: 300,
    completionTokens: 1_500,
    embeddingTokens: 0,
    totalTokens: 1_500,
  },
]

function mountTrend() {
  return mount(DashboardTrend, {
    props: {
      costCoverage: { status: 'complete' },
      daily,
      modelValue: 'requests',
    },
    global: {
      stubs: {
        AppSectionCard: {
          props: ['subtitle', 'title'],
          template:
            '<section><h2>{{ title }}</h2><p>{{ subtitle }}</p><slot name="actions" /><slot /></section>',
        },
        VBtn: {
          emits: ['click'],
          template: '<button @click="$emit(\'click\')"><slot /></button>',
        },
        VBtnToggle: { template: '<div><slot /></div>' },
        VChart: { template: '<div data-test="chart" />' },
        VIcon: true,
      },
    },
  })
}

describe('DashboardTrend', () => {
  beforeEach(() => {
    downloadDailyUsageCsvMock.mockReset()
  })

  it('keeps the export action inside the closed daily-data disclosure', () => {
    const wrapper = mountTrend()
    const details = wrapper.get('details')

    expect(details.attributes('open')).toBeUndefined()
    expect(details.get('summary').text()).toBe('View daily data')
    expect(details.get('[data-test="export-daily-data"]').text()).toBe(
      'Export CSV',
    )
  })

  it('exports the displayed daily metric with its date range', async () => {
    const wrapper = mountTrend()

    await wrapper.get('[data-test="export-daily-data"]').trigger('click')

    expect(downloadDailyUsageCsvMock).toHaveBeenCalledWith(
      daily,
      'requests',
      'promptgate-daily-requests-2026-10-23-to-2026-10-24.csv',
    )
  })
})
