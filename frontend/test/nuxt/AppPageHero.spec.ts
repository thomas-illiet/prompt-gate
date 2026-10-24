import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppPageHero from '../../app/components/App/AppPageHero.vue'

function mountHero(props = {}) {
  return mount(AppPageHero, {
    props: {
      copy: 'Keep an eye on the signals that matter.',
      kicker: 'Control room',
      statLabel: 'Current signal',
      statValue: 'Nominal',
      title: 'Usage overview',
      ...props,
    },
    global: {
      stubs: {
        VAvatar: { template: '<span><slot /></span>' },
        VCard: { template: '<section><slot /></section>' },
        VCardText: { template: '<div><slot /></div>' },
        VIcon: { template: '<i />' },
      },
    },
  })
}

describe('AppPageHero', () => {
  it('keeps the compact text-only hero when no illustration is provided', () => {
    const wrapper = mountHero()

    expect(wrapper.text()).toContain('Usage overview')
    expect(wrapper.text()).toContain('Nominal')
    expect(wrapper.find('[data-test="app-page-hero-visual"]').exists()).toBe(
      false,
    )
  })

  it('renders the requested vector visual as decoration', () => {
    const wrapper = mountHero({ visual: 'radar' })
    const visual = wrapper.get('[data-test="app-page-hero-visual"]')

    expect(visual.attributes('aria-hidden')).toBe('true')
    expect(visual.findComponent({ name: 'AppSignalVisual' }).exists()).toBe(
      true,
    )
  })
})
