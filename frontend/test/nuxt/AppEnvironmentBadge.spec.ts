import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppEnvironmentBadge from '~/components/App/AppEnvironmentBadge.vue'

describe('AppEnvironmentBadge', () => {
  it.each([
    ['development', 'Development', '#2878c8'],
    ['staging', 'Staging', '#d88216'],
    ['production', 'Production', '#d84b47'],
    ['unknown', 'Unknown', '#687580'],
  ] as const)(
    'renders the %s identity with text and its stable accent',
    (environment, label, accent) => {
      const wrapper = mount(AppEnvironmentBadge, {
        props: { environment },
      })

      expect(wrapper.text()).toBe(label)
      expect(wrapper.attributes('data-environment')).toBe(environment)
      expect(wrapper.attributes('style')).toContain(
        `--environment-accent: ${accent}`,
      )
    },
  )

  it('supports a quieter treatment for secondary placements', () => {
    const wrapper = mount(AppEnvironmentBadge, {
      props: { environment: 'development', subtle: true },
    })

    expect(wrapper.classes()).toContain('app-environment-badge--subtle')
    expect(wrapper.text()).toBe('Development')
  })
})
