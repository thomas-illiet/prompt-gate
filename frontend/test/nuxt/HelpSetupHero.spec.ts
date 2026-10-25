import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import HelpSetupHero from '../../app/components/HelpSetup/HelpSetupHero.vue'

function mountHero(props: { loading?: boolean; providerCount?: number } = {}) {
  return mount(HelpSetupHero, {
    props: {
      loading: false,
      providerCount: 0,
      ...props,
    },
    global: {
      stubs: {
        VAvatar: { template: '<span><slot /></span>' },
        VCard: {
          props: ['tag'],
          template: '<component :is="tag || \'div\'"><slot /></component>',
        },
        VIcon: { template: '<i />' },
      },
    },
  })
}

describe('HelpSetupHero', () => {
  it('presents the setup guide and the accessible provider count', () => {
    const wrapper = mountHero({ providerCount: 3 })
    const count = wrapper.get('[data-test="app-page-header-stat"]')
    const heading = wrapper.get('h1')

    expect(wrapper.findAll('h1')).toHaveLength(1)
    expect(heading.text()).toBe('Setup guide')
    expect(wrapper.get('header').attributes('aria-labelledby')).toBe(
      heading.attributes('id'),
    )
    expect(count.text()).toContain('Providers')
    expect(count.text()).toContain('3')
    expect(count.text()).toContain('3 routes ready')
    expect(count.attributes('role')).toBe('status')
    expect(count.attributes('aria-live')).toBe('polite')
    expect(count.attributes('aria-busy')).toBe('false')
    expect(wrapper.get('[aria-label="Supported setups"]').text()).toContain(
      'Coding agents',
    )
    expect(wrapper.get('[aria-label="Setup flow"]').text()).toContain(
      'Provider',
    )
  })

  it('announces an indeterminate provider count while loading', () => {
    const wrapper = mountHero({ loading: true, providerCount: 0 })
    const count = wrapper.get('[data-test="app-page-header-stat"]')

    expect(count.text()).toContain('…')
    expect(count.text()).toContain('Checking your routes')
    expect(count.attributes('aria-busy')).toBe('true')
  })

  it('keeps its illustration decorative', () => {
    const visual = mountHero().get('[data-test="app-page-header-visual"]')

    expect(visual.attributes('aria-hidden')).toBe('true')
    expect(visual.findComponent({ name: 'AppSignalVisual' }).exists()).toBe(
      true,
    )
  })
})
