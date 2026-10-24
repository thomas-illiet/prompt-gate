import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppEmptyState from '../../app/components/App/AppEmptyState.vue'

function mountEmptyState(props = {}) {
  return mount(AppEmptyState, {
    props: {
      text: 'Try another scope or come back after the next signal.',
      title: 'Nothing on the radar',
      ...props,
    },
    slots: {
      actions: '<button data-test="empty-action">Adjust scope</button>',
    },
    global: {
      stubs: {
        VAvatar: { template: '<span><slot /></span>' },
        VIcon: { template: '<i />' },
      },
    },
  })
}

describe('AppEmptyState', () => {
  it('keeps the radar fallback and action slot by default', () => {
    const wrapper = mountEmptyState()

    expect(wrapper.get('section').attributes('role')).toBe('status')
    expect(wrapper.get('section').attributes('aria-live')).toBe('polite')
    expect(wrapper.find('.app-empty-state__signal').exists()).toBe(true)
    expect(wrapper.get('[data-test="empty-action"]').text()).toBe(
      'Adjust scope',
    )
  })

  it('keeps the icon-only treatment in compact mode', () => {
    const wrapper = mountEmptyState({ compact: true, icon: 'mdi-key-outline' })
    expect(wrapper.classes()).toContain('app-empty-state--compact')
    expect(wrapper.find('.app-empty-state__signal').exists()).toBe(true)
  })
})
