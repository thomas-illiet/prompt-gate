import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent } from 'vue'

import QuotaExhaustedDialog from '../../app/components/QuotaExhaustion/QuotaExhaustedDialog.vue'
import type { ExhaustedQuotaWindow } from '../../app/composables/useQuotaExhaustionDialog'

const windows: ExhaustedQuotaWindow[] = [
  {
    key: '5h',
    label: '5-hour window',
    limit: 1_000,
    resetAt: '2026-10-24T15:00:00Z',
    used: 1_000,
  },
  {
    key: '7d',
    label: '7-day window',
    limit: 10_000,
    resetAt: '2026-10-25T15:00:00Z',
    used: 12_000,
  },
]

const DialogCardStub = defineComponent({
  props: {
    loading: Boolean,
    modelValue: Boolean,
    subtitle: String,
    title: String,
  },
  emits: ['update:modelValue'],
  template:
    '<section v-if="modelValue" data-test="dialog"><h2>{{ title }}</h2><p>{{ subtitle }}</p><button data-test="dialog-header-close" :disabled="loading" @click="$emit(\'update:modelValue\', false)">Close dialog</button><slot /><footer><slot name="actions" /></footer></section>',
})

const ActionButtonStub = defineComponent({
  props: {
    disabled: Boolean,
    label: String,
    loading: Boolean,
  },
  emits: ['click'],
  template:
    "<button :data-test=\"'action-' + String(label).toLowerCase().replaceAll(' ', '-')\" :disabled=\"disabled || loading\" @click=\"$emit('click')\">{{ label }}</button>",
})

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(QuotaExhaustedDialog, {
    props: {
      canReset: true,
      modelValue: true,
      resetAvailable: 2,
      resetError: null,
      resetLoading: false,
      resetMaximum: 2,
      resetPending: false,
      resetUsing: false,
      windows,
      ...props,
    },
    global: {
      stubs: {
        AppDialogActionButton: ActionButtonStub,
        AppDialogCard: DialogCardStub,
        AppDialogCloseButton: ActionButtonStub,
        VAlert: { template: '<aside><slot /></aside>' },
        VChip: { template: '<span><slot /></span>' },
        VProgressCircular: true,
      },
    },
  })
}

describe('QuotaExhaustedDialog', () => {
  it('shows every exhausted window with a decorative illustration', () => {
    const wrapper = mountDialog()

    expect(wrapper.text()).toContain('Quota reached')
    expect(wrapper.find('[data-test="quota-window-5h"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="quota-window-7d"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('1,000 / 1,000 tokens used')
    expect(wrapper.text()).toContain('12,000 / 10,000 tokens used')
    expect(wrapper.get('img').attributes('alt')).toBe('')
  })

  it('dismisses from the explicit close button and the dialog close path', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="action-close"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[false]])
    wrapper.unmount()

    const headerWrapper = mountDialog()
    await headerWrapper
      .get('[data-test="dialog-header-close"]')
      .trigger('click')
    expect(headerWrapper.emitted('update:modelValue')).toEqual([[false]])
    headerWrapper.unmount()
  })

  it('asks for confirmation before emitting the reset action', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="action-use-a-reset"]').trigger('click')

    expect(wrapper.text()).toContain('Use one quota reset?')
    expect(wrapper.text()).toContain('1 reset will remain.')
    expect(wrapper.emitted('reset')).toBeUndefined()

    await wrapper.get('[data-test="action-confirm-reset"]').trigger('click')
    expect(wrapper.emitted('reset')).toHaveLength(1)
  })

  it('hides the reset action and explains unavailable or pending state', async () => {
    const wrapper = mountDialog({
      canReset: false,
      resetAvailable: 0,
      resetError: 'Unable to load quota resets.',
    })

    expect(wrapper.find('[data-test="action-use-a-reset"]').exists()).toBe(
      false,
    )
    expect(wrapper.find('[data-test="quota-reset-unavailable"]').exists()).toBe(
      true,
    )

    await wrapper.setProps({ resetError: null, resetPending: true })
    expect(wrapper.find('[data-test="quota-reset-pending"]').exists()).toBe(
      true,
    )
  })
})
