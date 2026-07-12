import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppConfirmDialog from '../../app/components/App/AppConfirmDialog.vue'

function mountDialog(props = {}) {
  return mount(AppConfirmDialog, {
    props: {
      modelValue: true,
      title: 'Delete item',
      ...props,
    },
    slots: {
      default: '<strong>Custom body</strong>',
    },
    global: {
      stubs: {
        AppDialogCard: {
          props: ['loading', 'persistent'],
          template:
            '<section data-test="dialog" :data-persistent="loading || persistent"><slot /><footer><slot name="actions" /></footer></section>',
        },
        AppDialogActionButton: {
          emits: ['click'],
          props: ['label', 'loading'],
          template:
            '<button data-test="confirm" :disabled="loading" @click="$emit(\'click\')">{{ label }}</button>',
        },
        AppDialogCloseButton: {
          emits: ['click'],
          props: ['disabled', 'label'],
          template:
            '<button data-test="cancel" :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>',
        },
      },
    },
  })
}

describe('AppConfirmDialog', () => {
  it('renders slot body and emits confirm', async () => {
    const wrapper = mountDialog({ confirmLabel: 'Delete' })

    expect(wrapper.html()).toContain('Custom body')

    await wrapper.get('[data-test="confirm"]').trigger('click')

    expect(wrapper.emitted('confirm')).toHaveLength(1)
  })

  it('stays persistent and blocks cancel while loading', async () => {
    const wrapper = mountDialog({ loading: true })

    expect(wrapper.get('[data-test="dialog"]').attributes('data-persistent')).toBe(
      'true',
    )

    await wrapper.get('[data-test="cancel"]').trigger('click')

    expect(wrapper.emitted('cancel')).toBeUndefined()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
