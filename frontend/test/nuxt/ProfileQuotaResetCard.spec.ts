import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ProfileQuotaResetCard from '../../app/components/Profile/ProfileQuotaResetCard.vue'
import type { QuotaResetSummary } from '../../app/types/quota-resets'

const availableSummary: QuotaResetSummary = {
  available: 1,
  maximum: 2,
  hasResettableQuota: true,
  pendingReset: false,
}

function mountCard(
  summary: QuotaResetSummary | null = availableSummary,
  overrides: Partial<{
    error: string | null
    loading: boolean
    using: boolean
  }> = {},
) {
  return mount(ProfileQuotaResetCard, {
    props: {
      error: null,
      loading: false,
      summary,
      using: false,
      ...overrides,
    },
    global: {
      stubs: {
        ProfileInfoCard: {
          template: '<section><slot name="actions" /><slot /></section>',
        },
        VAlert: { template: '<div><slot /></div>' },
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: {
          props: ['disabled', 'loading'],
          emits: ['click'],
          template:
            '<button :disabled="disabled || loading" @click="$emit(\'click\')"><slot /></button>',
        },
        VChip: { template: '<span><slot /></span>' },
        VProgressCircular: { template: '<span>Loading</span>' },
      },
    },
  })
}

describe('ProfileQuotaResetCard', () => {
  it('does not invent a balance while the initial summary is loading', () => {
    const wrapper = mountCard(null, { loading: true })

    expect(wrapper.find('[data-test="quota-reset-balance-chip"]').exists()).toBe(
      false,
    )
    expect(wrapper.find('[data-test="quota-reset-available"]').exists()).toBe(
      false,
    )
    expect(wrapper.find('[aria-label="Loading quota resets"]').exists()).toBe(
      true,
    )
  })

  it('does not invent a balance when the initial summary fails', async () => {
    const wrapper = mountCard(null, {
      error: 'Unable to load quota resets.',
    })

    expect(wrapper.find('[data-test="quota-reset-balance-chip"]').exists()).toBe(
      false,
    )
    expect(wrapper.find('[data-test="quota-reset-available"]').exists()).toBe(
      false,
    )
    expect(wrapper.text()).toContain('Unable to load quota resets.')

    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('retry')).toEqual([[]])
  })

  it('shows the available balance and emits both user actions', async () => {
    const wrapper = mountCard()

    expect(wrapper.get('[data-test="quota-reset-balance-chip"]').text()).toBe(
      '1/2 available',
    )
    expect(wrapper.get('[data-test="quota-reset-available"]').text()).toBe('1')

    await wrapper.get('[data-test="use-quota-reset"]').trigger('click')
    await wrapper.get('[data-test="show-quota-reset-history"]').trigger('click')

    expect(wrapper.emitted('use')).toEqual([[]])
    expect(wrapper.emitted('showHistory')).toEqual([[]])
  })

  it.each([
    {
      label: 'the balance is empty',
      summary: { ...availableSummary, available: 0 },
    },
    {
      label: 'the current plan has no resettable quota',
      summary: { ...availableSummary, hasResettableQuota: false },
    },
    {
      label: 'a reset is pending',
      summary: { ...availableSummary, pendingReset: true },
    },
  ])('disables reset consumption when $label', ({ summary }) => {
    const wrapper = mountCard(summary)

    expect(
      wrapper.get('[data-test="use-quota-reset"]').attributes('disabled'),
    ).toBe('')
  })

  it('shows pending state while keeping history available', () => {
    const wrapper = mountCard({ ...availableSummary, pendingReset: true })

    expect(wrapper.get('[data-test="quota-reset-pending"]').text()).toContain(
      'update automatically',
    )
    expect(
      wrapper
        .get('[data-test="show-quota-reset-history"]')
        .attributes('disabled'),
    ).toBeUndefined()
  })

  it('offers a retry when the summary cannot be loaded', async () => {
    const wrapper = mountCard()
    await wrapper.setProps({ error: 'Unable to load quota resets.' })

    expect(wrapper.text()).toContain('Unable to load quota resets.')
    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('retry')).toEqual([[]])
  })

  it('offers a refresh when no summary state is available', async () => {
    const wrapper = mountCard(null)

    expect(wrapper.get('[data-test="quota-reset-unavailable"]').text()).toContain(
      'not available yet',
    )
    expect(wrapper.find('[data-test="quota-reset-balance-chip"]').exists()).toBe(
      false,
    )

    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('retry')).toEqual([[]])
  })
})
