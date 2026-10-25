import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, expect, it } from 'vitest'

import DashboardMetricSelect from '../../app/components/Dashboard/DashboardMetricSelect.vue'

const VBtnToggleStub = defineComponent({
  inheritAttrs: false,
  props: {
    color: String,
    density: String,
    mandatory: Boolean,
    modelValue: String,
    variant: String,
  },
  emits: ['update:modelValue'],
  template: '<div v-bind="$attrs"><slot /></div>',
})

const VBtnStub = defineComponent({
  inheritAttrs: false,
  props: {
    size: String,
    value: String,
  },
  template:
    '<button v-bind="$attrs" :data-size="size" :data-value="value"><slot /></button>',
})

describe('DashboardMetricSelect', () => {
  it('renders requests, tokens, and cost as one mandatory horizontal group', async () => {
    const wrapper = mount(DashboardMetricSelect, {
      props: {
        ariaLabel: 'Identity ranking metric',
        modelValue: 'requests',
      },
      global: {
        stubs: {
          VBtn: VBtnStub,
          VBtnToggle: VBtnToggleStub,
          VIcon: true,
        },
      },
    })

    const toggle = wrapper.getComponent(VBtnToggleStub)
    expect(toggle.attributes('aria-label')).toBe('Identity ranking metric')
    expect(toggle.classes()).toContain('dashboard-metric-select')
    expect(toggle.props()).toMatchObject({
      color: 'primary',
      density: 'compact',
      mandatory: true,
      modelValue: 'requests',
      variant: 'outlined',
    })
    expect(
      wrapper.findAll('button').map((button) => ({
        label: button.text(),
        value: button.attributes('data-value'),
      })),
    ).toEqual([
      { label: 'Requests', value: 'requests' },
      { label: 'Tokens', value: 'tokens' },
      { label: 'Cost', value: 'cost' },
    ])

    toggle.vm.$emit('update:modelValue', 'tokens')
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('update:modelValue')).toEqual([['tokens']])
  })
})
