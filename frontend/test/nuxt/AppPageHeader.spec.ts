import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppPageHeader from '../../app/components/App/AppPageHeader.vue'

function mountHeader(
  props: Record<string, unknown> = {},
  slots: Record<string, string> = {},
) {
  return mount(AppPageHeader, {
    props: {
      copy: 'Keep an eye on the signals that matter.',
      kicker: 'Control room',
      title: 'Usage overview',
      ...props,
    },
    slots,
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

describe('AppPageHeader', () => {
  it('renders one semantic page heading and optional composition slots', () => {
    const wrapper = mountHeader(
      { density: 'compact' },
      {
        footer: '<nav data-test="footer-slot">Page controls</nav>',
        meta: '<span data-test="meta-slot">Updated now</span>',
      },
    )

    const header = wrapper.get('header')
    const headings = wrapper.findAll('h1')

    expect(headings).toHaveLength(1)
    expect(headings[0]?.text()).toBe('Usage overview')
    expect(header.attributes('aria-labelledby')).toBe(
      headings[0]?.attributes('id'),
    )
    expect(wrapper.get('[data-test="meta-slot"]').text()).toBe('Updated now')
    expect(wrapper.get('[data-test="footer-slot"]').text()).toBe(
      'Page controls',
    )
  })

  it('renders an accessible live stat only when its contract is provided', () => {
    const withoutStat = mountHeader()
    const withStat = mountHeader({
      statBusy: true,
      statDetail: 'Across the selected period',
      statLabel: 'Current signal',
      statValue: 'Nominal',
    })
    const stat = withStat.get('[data-test="app-page-header-stat"]')

    expect(
      withoutStat.find('[data-test="app-page-header-stat"]').exists(),
    ).toBe(false)
    expect(stat.attributes('role')).toBe('status')
    expect(stat.attributes('aria-live')).toBe('polite')
    expect(stat.attributes('aria-busy')).toBe('true')
    expect(stat.text()).toContain('Current signal')
    expect(stat.text()).toContain('Nominal')
    expect(stat.text()).toContain('Across the selected period')
  })

  it('keeps the optional signal visual decorative', () => {
    const wrapper = mountHeader({ visual: 'radar' })
    const visual = wrapper.get('[data-test="app-page-header-visual"]')

    expect(visual.attributes('aria-hidden')).toBe('true')
    expect(visual.findComponent({ name: 'AppSignalVisual' }).exists()).toBe(
      true,
    )
  })
})
