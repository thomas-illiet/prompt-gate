import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppLimitedAccessBanner from '../../app/components/App/AppLimitedAccessBanner.vue'

function mountBanner() {
  return mount(AppLimitedAccessBanner, {
    global: {
      stubs: {
        VAlert: {
          inheritAttrs: false,
          props: ['closable', 'type', 'variant'],
          template: `
            <aside
              v-bind="$attrs"
              data-test="limited-access-banner"
              :data-closable="String(Boolean(closable))"
              :data-type="type"
              :data-variant="variant"
            >
              <slot />
            </aside>
          `,
        },
      },
    },
  })
}

describe('AppLimitedAccessBanner', () => {
  it('explains limited access and how to request additional rights', () => {
    const wrapper = mountBanner()
    const banner = wrapper.get('[data-test="limited-access-banner"]')

    expect(banner.text()).toMatch(/limited access/i)
    expect(banner.text()).toContain('Contact a site administrator')
    expect(banner.attributes('data-type')).toBe('info')
    expect(banner.attributes('data-closable')).toBe('false')
  })

  it('announces the persistent information without using an urgent alert', () => {
    const banner = mountBanner().get('[data-test="limited-access-banner"]')

    expect(banner.attributes('role')).toBe('status')
    expect(banner.attributes('aria-live')).toBe('polite')
  })
})
