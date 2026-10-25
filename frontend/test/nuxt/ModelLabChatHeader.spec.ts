import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ModelLabChatHeader from '../../app/components/ModelLab/ModelLabChatHeader.vue'
import type { ModelLabTarget } from '../../app/types/model-lab'

const targets: ModelLabTarget[] = ['alpha', 'beta', 'gamma'].map((model) => ({
  model,
  providerDisplayName: 'OpenAI Test',
  providerName: `openai-${model}`,
  providerType: 'openai',
  protocol: 'openai-chat',
}))

function mountHeader(disabled = false, selectedTargets = targets.slice(0, 1)) {
  return mount(ModelLabChatHeader, {
    props: {
      disabled,
      hasConversation: false,
      maxAllowedOutputTokens: 4096,
      maxOutputTokens: 1024,
      maxSelectedTargets: 2,
      modelValue: selectedTargets,
      targets,
      temperature: 0.7,
    },
    global: {
      stubs: {
        ModelLabGenerationSettingsDialog: {
          emits: ['apply', 'update:modelValue'],
          props: ['modelValue'],
          template:
            '<button v-if="modelValue" data-test="apply-dialog" @click="$emit(\'apply\', { maxOutputTokens: 2048, temperature: 0.4 })">Apply dialog</button>',
        },
        VBtn: {
          emits: ['click'],
          props: ['color', 'disabled', 'height', 'prependIcon', 'variant'],
          template:
            '<button :data-color="color" :data-height="height" :data-icon="prependIcon" :data-variant="variant" :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
        },
        VAutocomplete: {
          name: 'VAutocomplete',
          emits: ['update:modelValue'],
          props: {
            disabled: Boolean,
            hideDetails: Boolean,
            hint: String,
            items: { type: Array, default: () => [] },
            modelValue: { type: Array, default: () => [] },
            persistentHint: Boolean,
          },
          template:
            '<button data-test="select-models" :disabled="disabled" @click="$emit(\'update:modelValue\', items.map((item) => item.value))">Select models</button>',
        },
      },
    },
  })
}

describe('ModelLabChatHeader', () => {
  it('opens generation settings in a dialog and applies both values', async () => {
    const wrapper = mountHeader()

    const settingsButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Generation Settings'))
    expect(settingsButton).toBeDefined()
    expect(settingsButton?.attributes('data-variant')).toBe('outlined')
    await settingsButton?.trigger('click')
    await wrapper.get('[data-test="apply-dialog"]').trigger('click')

    expect(wrapper.emitted('update:max-output-tokens')).toEqual([[2048]])
    expect(wrapper.emitted('update:temperature')).toEqual([[0.4]])
  })

  it('prevents settings changes while a response is streaming', () => {
    const wrapper = mountHeader(true)
    const settingsButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Generation Settings'))

    expect((settingsButton?.element as HTMLButtonElement).disabled).toBe(true)
    expect(
      (wrapper.get('[data-test="select-models"]').element as HTMLButtonElement)
        .disabled,
    ).toBe(true)
  })

  it('styles New chat as a primary secondary action', () => {
    const wrapper = mountHeader()
    const newChatButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('New chat'))

    expect(newChatButton?.attributes('data-color')).toBe('primary')
    expect(newChatButton?.attributes('data-icon')).toBe(
      'mdi-message-plus-outline',
    )
    expect(newChatButton?.attributes('data-variant')).toBe('tonal')
  })

  it('keeps the model field and action buttons at the same compact height', () => {
    const wrapper = mountHeader()
    const modelSelect = wrapper.getComponent({ name: 'VAutocomplete' })
    const actionButtons = wrapper.findAll(
      '.model-lab-chat-header__actions button',
    )

    expect(modelSelect.props('hideDetails')).toBe(true)
    expect(modelSelect.props('hint')).toBeUndefined()
    expect(modelSelect.props('persistentHint')).toBe(false)
    expect(actionButtons).toHaveLength(2)
    expect(
      actionButtons.map((button) => button.attributes('data-height')),
    ).toEqual(['48', '48'])
  })

  it('emits at most two selected models in their chosen order', async () => {
    const wrapper = mountHeader()

    await wrapper.get('[data-test="select-models"]').trigger('click')

    expect(wrapper.emitted('update:model-value')).toEqual([
      [targets.slice(0, 2)],
    ])
  })

  it('disables unselected options once two models are selected', () => {
    const wrapper = mountHeader(false, targets.slice(0, 2))
    const items = wrapper
      .getComponent({ name: 'VAutocomplete' })
      .props('items') as Array<{ props: { disabled: boolean } }>

    expect(items.map((item) => item.props.disabled)).toEqual([
      false,
      false,
      true,
    ])
  })
})
