import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ProfileTokenUsageSection from '../../app/components/Profile/ProfileTokenUsageSection.vue'
import type { ProfileTokenUsageSummary } from '../../app/composables/useProfileTokenUsage'

function summary(
  overrides: Partial<ProfileTokenUsageSummary> = {},
): ProfileTokenUsageSummary {
  return {
    activeDays: 2,
    currentStreakDays: 1,
    days: [
      {
        date: '2026-06-02',
        requests: 1,
        prompts: 1,
        inputTokens: 100,
        outputTokens: 0,
        completionInputTokens: 100,
        completionOutputTokens: 0,
        completionTokens: 100,
        embeddingTokens: 0,
        totalTokens: 100,
      },
    ],
    endsAt: '2026-06-03',
    longestStreakDays: 2,
    maxTokens: 100,
    peakDay: {
      date: '2026-06-02',
      requests: 1,
      prompts: 1,
      inputTokens: 100,
      outputTokens: 0,
      completionInputTokens: 100,
      completionOutputTokens: 0,
      completionTokens: 100,
      embeddingTokens: 0,
      totalTokens: 100,
    },
    startsAt: '2025-06-04',
    totalTokens: 100,
    ...overrides,
  }
}

function mountSection(props: {
  error: string | null
  loading: boolean
  summary: ProfileTokenUsageSummary
}) {
  return mount(ProfileTokenUsageSection, {
    props,
    global: {
      stubs: {
        AppEmptyState: {
          props: ['title'],
          template: '<div data-test="empty-state">{{ title }}</div>',
        },
        ProfileInfoCard: {
          template:
            '<section><div data-test="actions"><slot name="actions" /></div><slot /></section>',
        },
        ProfileTokenHeatmap: {
          props: ['days'],
          template:
            '<div data-test="heatmap" :data-days="days.length">heatmap</div>',
        },
        VAlert: { template: '<div><slot /></div>' },
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: {
          emits: ['click'],
          template: '<button @click="$emit(\'click\')"><slot /></button>',
        },
        VChip: { template: '<span><slot /></span>' },
        VIcon: true,
      },
    },
  })
}

describe('ProfileTokenUsageSection', () => {
  it('renders the KPI cards and heatmap for active usage', () => {
    const wrapper = mountSection({
      error: null,
      loading: false,
      summary: summary(),
    })

    expect(wrapper.findAll('[data-test="profile-token-kpi"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('Tokens 12 mois')
    expect(wrapper.text()).toContain('Pic journalier')
    expect(wrapper.text()).toContain('Streak')
    expect(
      wrapper.get('[data-test="profile-token-heatmap"]').attributes('data-days'),
    ).toBe('1')
    expect(wrapper.get('[data-test="actions"]').text()).toContain('Daily')
    expect(wrapper.get('[data-test="actions"]').text()).toContain(
      '2 active days',
    )
  })

  it('renders loading, error retry, and empty states', async () => {
    const loadingWrapper = mountSection({
      error: null,
      loading: true,
      summary: summary(),
    })
    expect(loadingWrapper.find('[data-test="profile-token-loading"]').exists()).toBe(
      true,
    )

    const errorWrapper = mountSection({
      error: 'Unable to load usage',
      loading: false,
      summary: summary(),
    })
    await errorWrapper.get('button').trigger('click')
    expect(errorWrapper.text()).toContain('Unable to load usage')
    expect(errorWrapper.emitted('retry')).toHaveLength(1)

    const emptyWrapper = mountSection({
      error: null,
      loading: false,
      summary: summary({
        activeDays: 0,
        currentStreakDays: 0,
        longestStreakDays: 0,
        maxTokens: 0,
        peakDay: null,
        totalTokens: 0,
      }),
    })
    expect(emptyWrapper.get('[data-test="profile-token-empty"]').text()).toContain(
      'No token activity',
    )
  })
})
