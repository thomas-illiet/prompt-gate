import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import HelpSetupConfigurationCard from '../../app/components/HelpSetup/HelpSetupConfigurationCard.vue'
import type { HelpSetupProvider } from '../../app/types/user-service'

const openaiProvider: HelpSetupProvider = {
  name: 'openai-main',
  displayName: 'OpenAI Main',
  type: 'openai',
  routePrefix: '/openai-main/v1',
  openaiBaseUrl: 'https://proxy.example.com/openai-main/v1',
  models: ['gpt-4.1-mini', 'gpt-4.1'],
}

const anthropicProvider: HelpSetupProvider = {
  name: 'anthropic-main',
  displayName: 'Anthropic Main',
  type: 'anthropic',
  routePrefix: '/anthropic-main',
  anthropicBaseUrl: 'https://proxy.example.com/anthropic-main',
  models: [],
}

const global = {
  stubs: {
    AppSectionCard: {
      template: '<section><slot /></section>',
    },
    VAlert: { template: '<div><slot /></div>' },
    VChip: { template: '<span><slot /></span>' },
    VIcon: { template: '<span />' },
    VListItem: { template: '<div><slot /><slot name="subtitle" /></div>' },
    VSelect: {
      props: ['disabled', 'items', 'modelValue'],
      template:
        '<div class="select" :data-disabled="String(Boolean(disabled))" :data-model-value="modelValue"><span v-for="item in items" :key="item.value ?? item">{{ item.title ?? item }}</span></div>',
    },
  },
}

describe('HelpSetupConfigurationCard', () => {
  it('keeps model selection enabled for single-model snippets', () => {
    const wrapper = mount(HelpSetupConfigurationCard, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        modelOptions: openaiProvider.models,
        modelSelectMode: 'single',
        providerName: 'openai-main',
        providers: [openaiProvider],
        selectedProvider: openaiProvider,
      },
    })

    const selects = wrapper.findAll('.select')
    expect(selects).toHaveLength(2)
    const modelSelect = selects[1]!

    expect(modelSelect.attributes('data-disabled')).toBe('false')
    expect(modelSelect.attributes('data-model-value')).toBe('gpt-4.1-mini')
    expect(modelSelect.text()).toContain('gpt-4.1-mini')
    expect(modelSelect.text()).toContain('gpt-4.1')
  })

  it('shows a disabled All entry for multi-model snippets', () => {
    const wrapper = mount(HelpSetupConfigurationCard, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        modelOptions: openaiProvider.models,
        modelSelectMode: 'all',
        providerName: 'openai-main',
        providers: [openaiProvider],
        selectedProvider: openaiProvider,
      },
    })

    const selects = wrapper.findAll('.select')
    expect(selects).toHaveLength(2)
    const modelSelect = selects[1]!

    expect(modelSelect.attributes('data-disabled')).toBe('true')
    expect(modelSelect.attributes('data-model-value')).toBe('All')
    expect(modelSelect.text()).toContain('All')
  })

  it('labels Anthropic providers as not requiring a model', () => {
    const wrapper = mount(HelpSetupConfigurationCard, {
      global,
      props: {
        model: 'No model required',
        modelOptions: ['No model required'],
        modelSelectMode: 'none',
        providerName: 'anthropic-main',
        providers: [anthropicProvider],
        selectedProvider: anthropicProvider,
      },
    })

    const selects = wrapper.findAll('.select')
    expect(selects).toHaveLength(2)
    const modelSelect = selects[1]!

    expect(wrapper.text()).toContain('No model required')
    expect(wrapper.text()).not.toContain('0 models')
    expect(modelSelect.attributes('data-disabled')).toBe('true')
    expect(modelSelect.attributes('data-model-value')).toBe(
      'No model required',
    )
  })
})
