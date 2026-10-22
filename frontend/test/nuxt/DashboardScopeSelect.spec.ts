import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DashboardScopeSelect from '../../app/components/Dashboard/DashboardScopeSelect.vue'

describe('DashboardScopeSelect', () => {
  it('gives the select an accessible name', () => {
    const wrapper = mount(DashboardScopeSelect, {
      props: { modelValue: 'self' },
      global: {
        stubs: {
          VSelect: {
            inheritAttrs: false,
            props: ['items', 'modelValue'],
            emits: ['update:modelValue'],
            template:
              '<select data-test="scope-select" v-bind="$attrs" :value="modelValue" />',
          },
        },
      },
    })

    expect(
      wrapper.get('[data-test="scope-select"]').attributes('aria-label'),
    ).toBe('Dashboard scope')
  })
})
