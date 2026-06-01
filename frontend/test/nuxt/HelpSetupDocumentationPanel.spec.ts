import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { nextTick } from 'vue'

import HelpSetupDocumentationPanel from '../../app/components/HelpSetup/HelpSetupDocumentationPanel.vue'
import type { HelpSetupProvider } from '../../app/types/user-service'

const openaiProvider: HelpSetupProvider = {
  name: 'openai-main',
  displayName: 'OpenAI Main',
  type: 'openai',
  routePrefix: '/openai-main/v1',
  openaiBaseUrl: 'https://proxy.example.com/openai-main/v1',
  models: ['gpt-4.1-mini'],
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
    HelpSetupSnippetCard: {
      props: ['code', 'filePaths', 'subtitle', 'title'],
      template:
        '<section class="snippet"><h2>{{ title }}</h2><p>{{ subtitle }}</p><div class="paths"><span v-for="filePath in filePaths" :key="filePath">{{ filePath }}</span></div><div class="controls"><slot name="controls" /></div><pre><slot>{{ code }}</slot></pre></section>',
    },
    VList: { template: '<nav><slot /></nav>' },
    VListItem: {
      props: ['active', 'prependIcon', 'title', 'value'],
      emits: ['click'],
      template:
        '<button :data-active="active" :data-value="value" @click="$emit(\'click\')">{{ title }}</button>',
    },
    VAlert: { template: '<div><slot /></div>' },
    VSelect: { template: '<div />' },
  },
}

describe('HelpSetupDocumentationPanel', () => {
  it('shows OpenAI-compatible docs for OpenAI providers', () => {
    const wrapper = mount(HelpSetupDocumentationPanel, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })

    expect(wrapper.text()).toContain('curl')
    expect(wrapper.text()).toContain('Python')
    expect(wrapper.text()).toContain('Go')
    expect(wrapper.text()).toContain('ASP.NET')
    expect(wrapper.text()).toContain('Java')
    expect(wrapper.text()).toContain('PowerShell')
    expect(wrapper.text()).toContain('Lua')
    expect(wrapper.text()).toContain('Cline')
    expect(wrapper.text()).toContain('Continue')
    expect(wrapper.text()).toContain('OpenClaw')
    expect(wrapper.text()).toContain('OpenCode')
    expect(wrapper.text()).not.toContain('Codex')
    expect(wrapper.text()).not.toContain('Claude Code')
  })

  it('shows only Anthropic docs for Anthropic providers', async () => {
    const wrapper = mount(HelpSetupDocumentationPanel, {
      global,
      props: {
        model: 'No model required',
        provider: anthropicProvider,
      },
    })
    await nextTick()

    expect(wrapper.text()).toContain('Claude Code')
    expect(wrapper.text()).not.toContain('Codex')
    expect(wrapper.text()).not.toContain('Python')
    expect(wrapper.text()).not.toContain('Go')
    expect(wrapper.text()).not.toContain('ASP.NET')
    expect(wrapper.text()).not.toContain('Java')
    expect(wrapper.text()).not.toContain('PowerShell')
    expect(wrapper.text()).not.toContain('Lua')
    expect(wrapper.text()).not.toContain('Cline')
    expect(wrapper.text()).not.toContain('Continue')
    expect(wrapper.text()).not.toContain('OpenClaw')
    expect(wrapper.text()).not.toContain('OpenCode')
    expect(wrapper.text()).not.toContain('curl')
  })

  it('orders OpenAI-compatible docs by name', () => {
    const wrapper = mount(HelpSetupDocumentationPanel, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })

    expect(wrapper.findAll('button').map((button) => button.text())).toEqual([
      'ASP.NET',
      'Cline',
      'Continue',
      'curl',
      'Go',
      'Java',
      'Lua',
      'OpenClaw',
      'OpenCode',
      'PowerShell',
      'Python',
    ])
  })
})
