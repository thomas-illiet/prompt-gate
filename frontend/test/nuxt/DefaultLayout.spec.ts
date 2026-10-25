import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent } from 'vue'

import DefaultLayout from '../../app/layouts/default.vue'

const AuthenticatedShellStub = defineComponent({
  props: { mode: String },
  template:
    '<section data-test="authenticated-shell" :data-mode="mode"><slot /></section>',
})

describe('DefaultLayout', () => {
  it('renders page content in the user shell', () => {
    const wrapper = mount(DefaultLayout, {
      slots: { default: '<div data-test="page-content">Page content</div>' },
      global: {
        stubs: { AppAuthenticatedShell: AuthenticatedShellStub },
      },
    })

    expect(
      wrapper.get('[data-test="authenticated-shell"]').attributes('data-mode'),
    ).toBe('user')
    expect(wrapper.get('[data-test="page-content"]').text()).toBe(
      'Page content',
    )
  })
})
