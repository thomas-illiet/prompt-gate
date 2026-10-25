import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { nextTick, shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardAnnualActivity from '../../app/components/Dashboard/DashboardAnnualActivity.vue'
import type { ProfileTokenUsageSummary } from '../../app/composables/useProfileTokenUsage'

const { downloadDailyUsageCsvMock, useProfileTokenUsageMock } = vi.hoisted(
  () => ({
    downloadDailyUsageCsvMock: vi.fn(),
    useProfileTokenUsageMock: vi.fn(),
  }),
)

mockNuxtImport('useProfileTokenUsage', () => useProfileTokenUsageMock)

vi.mock('../../app/utils/daily-usage-csv', () => ({
  downloadDailyUsageCsv: downloadDailyUsageCsvMock,
}))

const usageDay: ProfileTokenUsageSummary['days'][number] = {
  date: '2026-10-24',
  requests: 4,
  inputTokens: 800,
  outputTokens: 200,
  completionInputTokens: 800,
  completionOutputTokens: 200,
  completionTokens: 1_000,
  embeddingTokens: 0,
  totalTokens: 1_000,
}

const annualSummary: ProfileTokenUsageSummary = {
  activeDays: 1,
  currentStreakDays: 1,
  days: [usageDay],
  endsAt: '2026-10-24',
  longestStreakDays: 1,
  maxTokens: 1_000,
  peakDay: usageDay,
  startsAt: '2025-10-25',
  totalTokens: 1_000,
}

function mountAnnualActivity(summary = annualSummary) {
  const reload = vi.fn()
  useProfileTokenUsageMock.mockReturnValue({
    activity: shallowRef({}),
    error: shallowRef(null),
    loading: shallowRef(false),
    reload,
    summary: shallowRef(summary),
  })

  const wrapper = mount(DashboardAnnualActivity, {
    props: { refreshVersion: 0 },
    global: {
      stubs: {
        AppSectionCard: {
          props: ['subtitle', 'title'],
          template:
            '<section><h2>{{ title }}</h2><p>{{ subtitle }}</p><div data-test="actions"><slot name="actions" /></div><slot /></section>',
        },
        ProfileTokenHeatmap: {
          props: ['days'],
          template: '<div data-test="heatmap" :data-days="days.length" />',
        },
        VAlert: { template: '<div><slot /></div>' },
        VBtn: {
          emits: ['click'],
          template: '<button @click="$emit(\'click\')"><slot /></button>',
        },
        VIcon: true,
      },
    },
  })

  return { reload, wrapper }
}

describe('DashboardAnnualActivity', () => {
  beforeEach(() => {
    downloadDailyUsageCsvMock.mockReset()
    useProfileTokenUsageMock.mockReset()
  })

  it('keeps the annual activity visible without a collapse control', () => {
    const { wrapper } = mountAnnualActivity()

    expect(useProfileTokenUsageMock.mock.calls[0]).toEqual([])
    expect(wrapper.get('h2').text()).toBe('Annual activity')
    expect(wrapper.text()).toContain('Total tokens')
    expect(wrapper.text()).toContain('1,000 over 365 days')
    expect(wrapper.get('[data-test="heatmap"]').attributes('data-days')).toBe(
      '1',
    )
    expect(wrapper.get('[data-test="actions"]').text()).toBe('')
    expect(wrapper.text()).not.toContain('Show annual activity')
    expect(wrapper.text()).not.toContain('Collapse')
  })

  it('reloads the visible activity when the dashboard refreshes', async () => {
    const { reload, wrapper } = mountAnnualActivity()

    await wrapper.setProps({ refreshVersion: 1 })
    await nextTick()

    expect(reload).toHaveBeenCalledOnce()
  })

  it('exports the daily rows from the expanded annual table', async () => {
    const { wrapper } = mountAnnualActivity()
    const details = wrapper.get('details')

    expect(details.attributes('open')).toBeUndefined()
    expect(details.get('[data-test="export-daily-data"]').text()).toBe(
      'Export CSV',
    )

    await details.get('[data-test="export-daily-data"]').trigger('click')

    expect(downloadDailyUsageCsvMock).toHaveBeenCalledWith(
      annualSummary.days,
      'annual',
      'promptgate-annual-activity-2026-10-24-to-2026-10-24.csv',
    )
  })

  it('sorts every annual data column and centers compact values', async () => {
    const earlierDay = {
      ...usageDay,
      date: '2026-10-23',
      requests: 2,
      totalTokens: 500,
    }
    const { wrapper } = mountAnnualActivity({
      ...annualSummary,
      activeDays: 2,
      days: [usageDay, earlierDay],
    })

    expect(wrapper.findAll('tbody tr').map((row) => row.text())).toEqual([
      '2026-10-232500',
      '2026-10-2441,000',
    ])
    const headers = wrapper.findAll('thead th')
    expect(headers.map((header) => header.attributes('aria-sort'))).toEqual([
      'ascending',
      undefined,
      undefined,
    ])
    expect(
      headers.every((header) =>
        header.classes().includes('app-table-sort-header--center'),
      ),
    ).toBe(true)

    await wrapper
      .get('button[aria-label="Sort by API requests"]')
      .trigger('click')
    expect(wrapper.findAll('tbody tr').map((row) => row.text())).toEqual([
      '2026-10-2441,000',
      '2026-10-232500',
    ])

    await wrapper.get('button[aria-label="Sort by Tokens"]').trigger('click')
    expect(wrapper.findAll('tbody tr').map((row) => row.text())).toEqual([
      '2026-10-2441,000',
      '2026-10-232500',
    ])
  })
})
