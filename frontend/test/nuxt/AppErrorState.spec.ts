import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { describe, expect, it } from 'vitest'

import AppErrorState from '../../app/components/App/AppErrorState.vue'

describe('AppErrorState', () => {
  it('renders a labelled image error state with actions', () => {
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
    const code = wrapper.get('.app-error-state__code')
    expect(code.get('span').text()).toBe('HTTP')
    expect(code.get('strong').text()).toBe('404')
    const image = wrapper.get('[data-test="app-error-image"]')
    expect(image.attributes('alt')).toBe('')
    expect(image.attributes('aria-hidden')).toBe('true')
    expect(image.attributes('data-variant')).toBe('not-found')
    expect(image.attributes('src')).toContain('error-not-found.jpg')
    expect(
      wrapper.get('.app-error-state__visual').attributes('aria-hidden'),
    ).toBeUndefined()
    expect(wrapper.text()).toContain('Back to dashboard')
  })

  it('focuses the alert and omits an empty actions region', async () => {
    const host = document.createElement('div')
    document.body.append(host)

    const wrapper = mount(AppErrorState, {
      attachTo: host,
      props: {
        code: 500,
        eyebrow: 'Control room signal',
        text: 'A safe public error message.',
        title: 'Something went off course',
      },
    })

    await nextTick()

    expect(wrapper.attributes('tabindex')).toBe('-1')
    expect(document.activeElement).toBe(wrapper.element)
    expect(wrapper.find('.app-error-state__actions').exists()).toBe(false)

    wrapper.unmount()
    host.remove()
  })

  it.each([
    ['forbidden', 'error-forbidden.jpg'],
    ['generic', 'error-generic.jpg'],
    ['not-found', 'error-not-found.jpg'],
    ['rate-limit', 'error-rate-limit.jpg'],
    ['server', 'error-server.jpg'],
  ] as const)('maps the %s variant to its dedicated image', (variant, file) => {
    const wrapper = mount(AppErrorState, {
      props: {
        code: 500,
        eyebrow: 'Control room signal',
        text: 'A safe public error message.',
        title: 'Something went off course',
        variant,
      },
    })

    const image = wrapper.get('[data-test="app-error-image"]')
    expect(image.attributes('data-variant')).toBe(variant)
    expect(image.attributes('src')).toContain(file)
  })
})
