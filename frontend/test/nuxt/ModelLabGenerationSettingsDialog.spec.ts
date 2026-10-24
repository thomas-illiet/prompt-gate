import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ModelLabGenerationSettingsDialog from '../../app/components/ModelLab/ModelLabGenerationSettingsDialog.vue'

function mountDialog(modelValue = true) {
  return mount(ModelLabGenerationSettingsDialog, {
    props: {
      maxAllowedOutputTokens: 4096,
      maxOutputTokens: 1024,
      modelValue,
      temperature: 0.7,
    },
    global: {
      stubs: {
        AppDialogActionButton: {
          emits: ['click'],
          props: ['form', 'label', 'type'],
          template:
            '<button :data-test="label" :form="form" :type="type" @click="$emit(\'click\')">{{ label }}</button>',
        },
        AppDialogCard: {
          props: ['modelValue'],
          template:
            '<section v-if="modelValue"><slot /><slot name="actions" /></section>',
        },
        AppDialogCloseButton: {
          emits: ['click'],
          props: ['label'],
          template:
            '<button :data-test="label" @click="$emit(\'click\')">{{ label }}</button>',
        },
        VNumberInput: {
          emits: ['update:modelValue'],
          template:
            '<button data-test="set-output-tokens" @click="$emit(\'update:modelValue\', 2048)">Set tokens</button>',
        },
        VSlider: {
          emits: ['update:modelValue'],
          template:
            '<button data-test="set-temperature" @click="$emit(\'update:modelValue\', 0.9)">Set temperature</button>',
        },
      },
    },
  })
}

describe('ModelLabGenerationSettingsDialog', () => {
  it('applies the draft settings only after confirmation', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="set-temperature"]').trigger('click')
    await wrapper.get('[data-test="set-output-tokens"]').trigger('click')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('apply')).toEqual([
      [{ maxOutputTokens: 2048, temperature: 0.9 }],
    ])
    expect(wrapper.emitted('update:modelValue')).toContainEqual([false])
  })

  it('discards the draft when cancelled', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="set-temperature"]').trigger('click')
    await wrapper.get('[data-test="set-output-tokens"]').trigger('click')
    await wrapper.get('[data-test="Cancel"]').trigger('click')

    expect(wrapper.emitted('apply')).toBeUndefined()
    expect(wrapper.emitted('update:modelValue')).toContainEqual([false])
  })

  it('reloads the current settings every time it opens', async () => {
    const wrapper = mountDialog(false)
    await wrapper.setProps({
      maxOutputTokens: 3072,
      modelValue: true,
      temperature: 0.3,
    })

    expect(wrapper.text()).toContain('0.3')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('apply')).toEqual([
      [{ maxOutputTokens: 3072, temperature: 0.3 }],
    ])
  })

  it('normalizes draft values to the supported bounds', async () => {
    const wrapper = mount(ModelLabGenerationSettingsDialog, {
      props: {
        maxAllowedOutputTokens: 4096,
        maxOutputTokens: 1024,
        modelValue: true,
        temperature: 0.7,
      },
      global: {
        stubs: {
          AppDialogActionButton: {
            emits: ['click'],
            props: ['form', 'label', 'type'],
            template:
              '<button data-test="apply" :form="form" :type="type" @click="$emit(\'click\')">{{ label }}</button>',
          },
          AppDialogCard: {
            props: ['modelValue'],
            template:
              '<section v-if="modelValue"><slot /><slot name="actions" /></section>',
          },
          AppDialogCloseButton: true,
          VNumberInput: {
            emits: ['update:modelValue'],
            template:
              '<button data-test="tokens-over-max" @click="$emit(\'update:modelValue\', 9000.4)">Tokens</button>',
          },
          VSlider: {
            emits: ['update:modelValue'],
            template:
              '<button data-test="temperature-over-max" @click="$emit(\'update:modelValue\', 4)">Temperature</button>',
          },
        },
      },
    })

    await wrapper.get('[data-test="tokens-over-max"]').trigger('click')
    await wrapper.get('[data-test="temperature-over-max"]').trigger('click')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('apply')).toEqual([
      [{ maxOutputTokens: 4096, temperature: 1 }],
    ])
  })
})
