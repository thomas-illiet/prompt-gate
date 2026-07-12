import { mount } from '@vue/test-utils'
import { computed, nextTick } from 'vue'
import { describe, expect, it } from 'vitest'

import AdminPricingModelDialog from '../../app/components/AdminPricing/AdminPricingModelDialog.vue'
import type { ModelPriceRecord } from '../../app/types/pricing'
import type { Provider, ProviderModelCatalog } from '../../app/types/providers'

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

const modelCatalog: ProviderModelCatalog[] = [
  {
    id: provider.id,
    name: provider.name,
    displayName: provider.displayName,
    models: ['gpt-4.1', 'gpt-5'],
  },
]

const existingPrice: ModelPriceRecord = {
  id: 'price-id',
  providerName: provider.name,
  model: 'gpt-4.1',
  input: 3,
  output: 4,
}

function mountDialog(price: ModelPriceRecord | null = null) {
  return mount(AdminPricingModelDialog, {
    props: {
      existingPrices: [existingPrice],
      loading: false,
      modelCatalog,
      modelValue: true,
      optionsLoading: false,
      price,
      providers: [provider],
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
        VCol: { template: '<div><slot /></div>' },
        VDialog: {
          props: ['modelValue'],
          template: '<div v-if="modelValue"><slot /></div>',
        },
        VRow: { template: '<div><slot /></div>' },
        VSelect: {
          emits: ['update:modelValue'],
          props: ['disabled', 'errorMessages', 'items', 'label', 'modelValue'],
          setup(props: {
            items?: Array<string | { title: string; value: string }>
          }) {
            const normalizedItems = computed(() =>
              (props.items ?? []).map((item) =>
                typeof item === 'string'
                  ? { title: item, value: item }
                  : { title: item.title, value: item.value },
              ),
            )
            return { normalizedItems }
          },
          template: `
            <label>
              <select
                :data-test="'field-' + label"
                :disabled="disabled"
                :value="modelValue"
                @change="$emit('update:modelValue', $event.target.value)"
              >
                <option value=""></option>
                <option
                  v-for="item in normalizedItems"
                  :key="item.value"
                  :value="item.value"
                >
                  {{ item.title }}
                </option>
              </select>
              <span v-for="message in errorMessages" :key="message">{{ message }}</span>
            </label>
          `,
        },
        VSpacer: { template: '<span />' },
        VTextField: {
          emits: ['update:modelValue'],
          props: ['errorMessages', 'label', 'modelValue', 'type'],
          template: `
            <label>
              <input
                :data-test="'field-' + label"
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

describe('AdminPricingModelDialog', () => {
  it('creates a model price from existing provider and unpriced model options', async () => {
    const wrapper = mountDialog()

    await wrapper
      .get('[data-test="field-Provider name"]')
      .setValue(provider.name)
    await nextTick()

    const modelOptions = wrapper
      .get('[data-test="field-Model"]')
      .findAll('option')
      .map((option) => option.element.value)
      .filter(Boolean)

    expect(modelOptions).toEqual(['gpt-5'])

    await wrapper.get('[data-test="field-Model"]').setValue('gpt-5')
    await wrapper.get('[data-test="field-Input USD / 1M tokens"]').setValue('1')
    await wrapper
      .get('[data-test="field-Output USD / 1M tokens"]')
      .setValue('2')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toEqual([
      [
        {
          providerName: provider.name,
          model: 'gpt-5',
          input: 1,
          output: 2,
        },
      ],
    ])
  })

  it('keeps the current model selectable while editing an existing price', async () => {
    const wrapper = mountDialog(existingPrice)

    await nextTick()

    const modelOptions = wrapper
      .get('[data-test="field-Model"]')
      .findAll('option')
      .map((option) => option.element.value)
      .filter(Boolean)

    expect(modelOptions).toEqual(['gpt-4.1', 'gpt-5'])

    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({
      providerName: provider.name,
      model: 'gpt-4.1',
    })
  })

  it('disables provider and model fields while editing an existing price', async () => {
    const wrapper = mountDialog(existingPrice)

    await nextTick()

    expect(
      (
        wrapper.get('[data-test="field-Provider name"]')
          .element as HTMLSelectElement
      ).disabled,
    ).toBe(true)
    expect(
      (wrapper.get('[data-test="field-Model"]').element as HTMLSelectElement)
        .disabled,
    ).toBe(true)
  })

  it('clamps negative edit prices to zero before saving', async () => {
    const wrapper = mountDialog({
      ...existingPrice,
      input: -3,
      output: -4,
    })

    await nextTick()
    await wrapper
      .get('[data-test="field-Input USD / 1M tokens"]')
      .setValue('-5')
    await wrapper
      .get('[data-test="field-Output USD / 1M tokens"]')
      .setValue('-7')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({
      input: 0,
      output: 0,
    })
  })
})
