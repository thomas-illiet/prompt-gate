import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ModelLabChatComposer from '../../app/components/ModelLab/ModelLabChatComposer.vue'

function mountComposer(options: { canSend?: boolean; running?: boolean } = {}) {
  return mount(ModelLabChatComposer, {
    props: {
      canSend: options.canSend ?? true,
      error: '',
      maxPromptLength: 8192,
      modelValue: 'Hello',
      running: options.running ?? false,
    },
    global: {
      stubs: {
        VBtn: {
          emits: ['click'],
          props: ['disabled', 'height'],
          template:
            '<button :data-height="height" :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
        },
        VTextField: {
          name: 'VTextField',
          emits: ['keydown', 'update:modelValue'],
          methods: { focus() {} },
          props: ['modelValue', 'type'],
          template:
            '<input data-test="composer-input" :type="type" :value="modelValue" @keydown="$emit(\'keydown\', $event)" />',
        },
      },
    },
  })
}

describe('ModelLabChatComposer', () => {
  it('uses a single-line input and sends with Enter', async () => {
    const wrapper = mountComposer()
    const input = wrapper.get('[data-test="composer-input"]')

    expect(input.element.tagName).toBe('INPUT')
    expect(input.attributes('type')).toBe('text')
    expect(wrapper.get('button').attributes('data-height')).toBe('56')

    await input.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('send')).toHaveLength(1)

    await input.trigger('keydown', { key: 'Enter', shiftKey: true })
    expect(wrapper.emitted('send')).toHaveLength(2)
  })

  it('does not submit an IME composition while sending is otherwise allowed', async () => {
    const wrapper = mountComposer()
    const input = wrapper.get('[data-test="composer-input"]')

    await input.trigger('keydown', { isComposing: true, key: 'Enter' })
    expect(wrapper.emitted('send')).toBeUndefined()
  })

  it('exposes Stop while running', async () => {
    const wrapper = mountComposer({ running: true })

    expect(wrapper.get('button').attributes('data-height')).toBe('56')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('stop')).toHaveLength(1)
  })
})
