import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminProviderDialog from '../../app/components/AdminProviders/AdminProviderDialog.vue'
import type { Provider } from '../../app/types/providers'

const provider: Provider = {
  id: 'provider-id',
  name: 'openai-main',
  displayName: 'OpenAI Main',
  type: 'openai',
  baseUrl: 'https://api.openai.com/v1',
  hasApiKey: true,
  enabled: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function mountDialog(providerOverride: Provider | null = provider) {
  return mount(AdminProviderDialog, {
    props: {
      loading: false,
      modelValue: true,
      provider: providerOverride,
    },
    global: {
      stubs: {
        AppDialogCard: {
          template: '<section><slot /><footer><slot name="actions" /></footer></section>',
        },
        AppDialogActionButton: {
          props: ['label', 'loading', 'type'],
          template:
            '<button data-test="submit" :type="type" :disabled="loading">{{ label }}</button>',
        },
        AppDialogCloseButton: {
          template: '<button type="button">Cancel</button>',
        },
        VCard: { template: '<section><slot /></section>' },
        VCardActions: { template: '<div><slot /></div>' },
        VCardText: { template: '<div><slot /></div>' },
        VCardTitle: { template: '<h2><slot /></h2>' },
        VCheckbox: {
          emits: ['update:modelValue'],
          props: ['label', 'modelValue'],
          template:
            '<label><input :data-test="`field-${label}`" type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" /></label>',
        },
        VCol: { template: '<div><slot /></div>' },
        VDialog: {
          props: ['modelValue'],
          template: '<div v-if="modelValue"><slot /></div>',
        },
        VRow: { template: '<div><slot /></div>' },
        VSelect: {
          emits: ['update:modelValue'],
          props: ['label', 'modelValue'],
          template:
            '<select :data-test="`field-${label}`" :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option :value="modelValue">{{ modelValue }}</option></select>',
        },
        VSpacer: { template: '<span />' },
        VTextField: {
          emits: ['update:modelValue'],
          props: [
            'disabled',
            'errorMessages',
            'label',
            'modelValue',
            'readonly',
            'type',
          ],
          template: `
            <label>
              <input
                :data-test="'field-' + label"
                :disabled="disabled"
                :readonly="readonly"
                :type="type || 'text'"
                :value="modelValue"
                @input="$emit('update:modelValue', $event.target.value)"
              />
              <span v-for="message in errorMessages" :key="message">{{ message }}</span>
            </label>
          `,
        },
      },
    },
  })
}

describe('AdminProviderDialog', () => {
  it('keeps provider name readonly in edit mode and omits it from update payloads', async () => {
    const wrapper = mountDialog()

    const nameInput = wrapper.get<HTMLInputElement>('[data-test="field-Name"]')
    expect(nameInput.element.value).toBe('openai-main')
    expect(nameInput.attributes('readonly')).toBeDefined()

    await wrapper
      .get('[data-test="field-Display name"]')
      .setValue('OpenAI Primary')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toEqual([
      [
        {
          displayName: 'OpenAI Primary',
          type: 'openai',
          baseUrl: 'https://api.openai.com/v1',
          enabled: true,
        },
      ],
    ])
  })
})
