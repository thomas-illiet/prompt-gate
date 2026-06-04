import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ProfileTokenHeatmap from '../../app/components/Profile/ProfileTokenHeatmap.vue'
import type { ProfileTokenUsageDay } from '../../app/composables/useProfileTokenUsage'

const DAY_MS = 24 * 60 * 60 * 1000

function profileDay(date: string, totalTokens: number): ProfileTokenUsageDay {
  return {
    date,
    requests: totalTokens > 0 ? 1 : 0,
    prompts: totalTokens > 0 ? 1 : 0,
    inputTokens: totalTokens,
    outputTokens: 0,
    completionInputTokens: totalTokens,
    completionOutputTokens: 0,
    completionTokens: totalTokens,
    embeddingTokens: 0,
    totalTokens,
  }
}

function dateKey(date: Date) {
  return date.toISOString().slice(0, 10)
}

function profileDays(
  startDate: string,
  tokensByIndex: (index: number) => number,
  count: number,
) {
  const start = new Date(`${startDate}T00:00:00Z`)

  return Array.from({ length: count }, (_, index) =>
    profileDay(dateKey(new Date(start.getTime() + index * DAY_MS)), tokensByIndex(index)),
  )
}

function mountHeatmap(days: ProfileTokenUsageDay[]) {
  return mount(ProfileTokenHeatmap, {
    props: { days },
    global: {
      stubs: {
        VTooltip: {
          props: ['text'],
          template:
            '<span :data-tooltip="text"><slot name="activator" :props="{}" /></span>',
        },
      },
    },
  })
}

describe('ProfileTokenHeatmap', () => {
  it('renders one day cell per profile day and aligns them into weeks', () => {
    const wrapper = mountHeatmap(
      profileDays('2025-06-04', (index) => (index % 11) * 10, 365),
    )

    expect(wrapper.findAll('[data-test="token-heatmap-cell"]')).toHaveLength(
      365,
    )
    expect(wrapper.findAll('[data-test="token-heatmap-month"]')).toHaveLength(
      53,
    )

    const monthLabels = wrapper
      .findAll('[data-test="token-heatmap-month"]')
      .map((label) => label.text())
      .filter(Boolean)

    expect(monthLabels).toContain('Jun')
    expect(monthLabels).toContain('Jul')
    expect(monthLabels).toContain('Jan')
  })

  it('assigns token intensity levels and tooltip labels', () => {
    const wrapper = mountHeatmap(
      profileDays(
        '2026-01-01',
        (index) => [0, 25, 50, 75, 100][index] ?? 0,
        5,
      ),
    )
    const cells = wrapper.findAll('[data-test="token-heatmap-cell"]')

    expect(cells.map((cell) => cell.attributes('data-level'))).toEqual([
      '0',
      '1',
      '2',
      '3',
      '4',
    ])
    expect(cells[4]?.attributes('title')).toContain('100 tokens')
    expect(cells[4]?.attributes('aria-label')).toContain('1 message')
  })
})
