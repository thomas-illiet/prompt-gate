import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppErrorState from '../../app/components/App/AppErrorState.vue'

describe('AppErrorState', () => {
  it('renders a labelled SVG error state with actions', () => {
    const wrapper = mount(AppErrorState, {
      props: {
        code: 404,
        eyebrow: 'Route signal',
        text: 'This route is not connected.',
        title: 'Page not found',
        variant: 'not-found',
      },
      slots: {
        actions: '<button>Back to dashboard</button>',
      },
    })

    expect(wrapper.get('article').attributes('aria-labelledby')).toBe(
      'app-error-title-404',
    )
    expect(wrapper.get('article').attributes('role')).toBe('alert')
    expect(wrapper.get('article').attributes('aria-live')).toBe('assertive')
    expect(wrapper.get('h1').text()).toBe('Page not found')
    expect(wrapper.text()).toContain('HTTP 404')
    expect(wrapper.get('svg').attributes('aria-hidden')).toBe('true')
    expect(
      wrapper.get('.app-error-state__visual').attributes('aria-hidden'),
    ).toBeUndefined()
    expect(wrapper.text()).toContain('Back to dashboard')
  })
})
