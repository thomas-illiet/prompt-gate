import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminGroupDialog from '../../app/components/AdminGroups/AdminGroupDialog.vue'
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

function mountDialog() {
  return mount(AdminGroupDialog, {
    props: {
      group: null,
      loading: false,
      modelValue: true,
      modelValidation: null,
      modelValidationError: null,
      modelValidationLoading: false,
      providers: [provider],
    },
    global: {
      stubs: {
        AppDialogActionButton: {
          props: ['label', 'loading', 'type'],
          template:
            '<button data-test="submit" :type="type" :disabled="loading">{{ label }}</button>',
        },
        AppDialogCloseButton: {
          template: '<button type="button">Cancel</button>',
        },
        VAlert: { template: '<section><slot /></section>' },
        VBtn: {
          emits: ['click'],
          props: ['disabled', 'loading'],
          template:
            '<button type="button" :disabled="disabled || loading" @click="$emit(\'click\')"><slot /></button>',
        },
        VCard: { template: '<section><slot /></section>' },
        VCardActions: { template: '<div><slot /></div>' },
        VCardText: { template: '<div><slot /></div>' },
        VCardTitle: { template: '<h2><slot /></h2>' },
        VCol: { template: '<div><slot /></div>' },
        VCombobox: {
          emits: ['update:modelValue'],
          props: ['modelValue'],
          template:
            '<input data-test="field-Allowed model regex" :value="modelValue.join(\',\')" @input="$emit(\'update:modelValue\', $event.target.value.split(\',\'))" />',
        },
        VDialog: {
          props: ['modelValue'],
          template: '<div v-if="modelValue"><slot /></div>',
        },
        VRow: { template: '<div><slot /></div>' },
        VSelect: {
          emits: ['update:modelValue'],
          props: ['errorMessages', 'modelValue'],
          template: `
            <div>
              <button data-test="select-provider" type="button" @click="$emit('update:modelValue', ['provider-id'])">Select provider</button>
              <span v-for="message in errorMessages" :key="message">{{ message }}</span>
            </div>
          `,
        },
        VSpacer: { template: '<span />' },
        VTextarea: {
          emits: ['update:modelValue'],
          props: ['modelValue'],
          template:
            '<textarea data-test="field-Description" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
        },
        VTextField: {
          emits: ['update:modelValue'],
          props: ['errorMessages', 'label', 'modelValue'],
          template: `
            <label>
              <input
                :data-test="'field-' + label"
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

describe('AdminGroupDialog', () => {
  it('autofills display name from name and saves the default all-model regex', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="field-Name"]').setValue('platform-team')
    await wrapper.get('[data-test="select-provider"]').trigger('click')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toEqual([
      [
        {
          name: 'platform-team',
          displayName: 'Platform Team',
          description: '',
          providerIds: ['provider-id'],
          modelPatterns: ['.*'],
        },
      ],
    ])
  })

  it('keeps a user-provided display name override', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="field-Name"]').setValue('platform-team')
    await wrapper
      .get('[data-test="field-Display name"]')
      .setValue('Core Platform')
    await wrapper.get('[data-test="select-provider"]').trigger('click')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({
      displayName: 'Core Platform',
    })
  })

  it('requires display name and one provider before saving', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="field-Name"]').setValue('platform')
    await wrapper.get('[data-test="field-Display name"]').setValue('')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toBeUndefined()
    expect(wrapper.text()).toContain('Display name is required.')
    expect(wrapper.text()).toContain('Select at least one provider.')
  })
})
