import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import DashboardPageHeader from '../../app/components/Dashboard/DashboardPageHeader.vue'

function mountHeader(props: Record<string, unknown> = {}) {
  return mount(DashboardPageHeader, {
    props: {
      copy: 'Your recent PromptGate activity.',
      isAdmin: true,
      loading: false,
      periodLabel: 'Oct 15 – Oct 22',
      scope: 'self',
      title: 'My usage',
      updatedAt: '2026-10-22T10:00:00Z',
      window: '7d',
      ...props,
    },
    global: {
      stubs: {
        AppSignalVisual: { template: '<span data-test="radar" />' },
        DashboardScopeSelect: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template:
            '<button data-test="scope" @click="$emit(\'update:modelValue\', \'global\')">{{ modelValue }}</button>',
        },
        DashboardTimeRangeSelect: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template:
            '<button data-test="window" @click="$emit(\'update:modelValue\', \'30d\')">{{ modelValue }}</button>',
        },
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: {
          inheritAttrs: false,
          props: ['loading'],
          emits: ['click'],
          template:
            '<button v-bind="$attrs" :data-loading="loading" @click="$emit(\'click\')"><slot /></button>',
        },
        VCard: {
          props: ['tag'],
          template: '<component :is="tag || \'div\'"><slot /></component>',
        },
        VIcon: { template: '<i />' },
      },
    },
  })
}

describe('DashboardPageHeader', () => {
  it('composes the dashboard identity, period, update time, and radar visual', () => {
    const wrapper = mountHeader()

    expect(wrapper.get('h1').text()).toBe('My usage')
    expect(wrapper.text()).toContain('Your recent PromptGate activity.')
    expect(wrapper.text()).toContain('Oct 15 – Oct 22')
    expect(wrapper.text()).toContain('Updated')
    expect(wrapper.get('[data-test="app-page-header-visual"]')).toBeDefined()
    expect(wrapper.get('[data-test="radar"]')).toBeDefined()
  })

  it('exposes typed scope and window models through the header controls', async () => {
    const updateScope = vi.fn()
    const updateWindow = vi.fn()
    const wrapper = mountHeader({
      'onUpdate:scope': updateScope,
      'onUpdate:window': updateWindow,
    })

    await wrapper.get('[data-test="scope"]').trigger('click')
    await wrapper.get('[data-test="window"]').trigger('click')

    expect(updateScope).toHaveBeenCalledWith('global')
    expect(updateWindow).toHaveBeenCalledWith('30d')
  })

  it('hides the scope control for non-admins and emits refresh', async () => {
    const wrapper = mountHeader({ isAdmin: false, loading: true })
    const refresh = wrapper.get('[aria-label="Refresh all dashboard data"]')

    expect(wrapper.find('[data-test="scope"]').exists()).toBe(false)
    expect(refresh.attributes('data-loading')).toBe('true')

    await refresh.trigger('click')

    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })
})
