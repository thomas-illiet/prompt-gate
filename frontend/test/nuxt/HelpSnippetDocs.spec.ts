import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import HelpSetupAspNetSnippet from '../../app/components/HelpSetup/docs/HelpSetupAspNetSnippet.vue'
import HelpSetupClaudeCodeSnippet from '../../app/components/HelpSetup/docs/HelpSetupClaudeCodeSnippet.vue'
import HelpSetupClineSnippet from '../../app/components/HelpSetup/docs/HelpSetupClineSnippet.vue'
import HelpSetupContinueSnippet from '../../app/components/HelpSetup/docs/HelpSetupContinueSnippet.vue'
import HelpSetupCurlSnippet from '../../app/components/HelpSetup/docs/HelpSetupCurlSnippet.vue'
import HelpSetupGoSnippet from '../../app/components/HelpSetup/docs/HelpSetupGoSnippet.vue'
import HelpSetupJavaSnippet from '../../app/components/HelpSetup/docs/HelpSetupJavaSnippet.vue'
import HelpSetupLuaSnippet from '../../app/components/HelpSetup/docs/HelpSetupLuaSnippet.vue'
import HelpSetupOpenClawSnippet from '../../app/components/HelpSetup/docs/HelpSetupOpenClawSnippet.vue'
import HelpSetupOpenCodeSnippet from '../../app/components/HelpSetup/docs/HelpSetupOpenCodeSnippet.vue'
import HelpSetupPowerShellSnippet from '../../app/components/HelpSetup/docs/HelpSetupPowerShellSnippet.vue'
import HelpSetupPythonSnippet from '../../app/components/HelpSetup/docs/HelpSetupPythonSnippet.vue'
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
  models: ['claude-3-7-sonnet-latest'],
}

const global = {
  stubs: {
    HelpSetupSnippetCard: {
      props: ['code', 'filePaths', 'subtitle', 'title'],
      template:
        '<section class="snippet"><h2>{{ title }}</h2><p>{{ subtitle }}</p><div class="paths"><span v-for="filePath in filePaths" :key="filePath">{{ filePath }}</span></div><div class="controls"><slot name="controls" /></div><pre><slot>{{ code }}</slot></pre></section>',
    },
    VAlert: { template: '<div><slot /></div>' },
  },
}

describe('help snippet documentation components', () => {
  it('renders OpenAI curl documentation', () => {
    const wrapper = mount(HelpSetupCurlSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })

    expect(wrapper.text()).toContain(
      'https://proxy.example.com/openai-main/v1/chat/completions',
    )
    expect(wrapper.text()).toContain('YOUR_PROMPTGATE_TOKEN')
  })

  it('renders Anthropic Claude Code documentation', () => {
    const wrapper = mount(HelpSetupClaudeCodeSnippet, {
      global,
      props: {
        model: 'claude-3-7-sonnet-latest',
        provider: anthropicProvider,
      },
    })

    expect(wrapper.text()).toContain('ANTHROPIC_BASE_URL')
    expect(wrapper.text()).toContain('YOUR_PROMPTGATE_TOKEN')
  })

  it('renders SDK and script documentation from components', () => {
    const python = mount(HelpSetupPythonSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })
    const go = mount(HelpSetupGoSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })
    const aspnet = mount(HelpSetupAspNetSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })
    const java = mount(HelpSetupJavaSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })
    const powershell = mount(HelpSetupPowerShellSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })
    const lua = mount(HelpSetupLuaSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })

    expect(python.text()).toContain('from openai import OpenAI')
    expect(python.text()).toContain('base_url')
    expect(python.text()).toContain('YOUR_PROMPTGATE_TOKEN')
    expect(python.text()).toContain('gpt-4.1-mini')

    expect(go.text()).toContain('option.WithBaseURL')
    expect(go.text()).toContain('option.WithAPIKey')
    expect(go.text()).toContain('gpt-4.1-mini')

    expect(aspnet.text()).toContain('WebApplication')
    expect(aspnet.text()).toContain('PostAsJsonAsync')
    expect(aspnet.text()).toContain('/chat/completions')
    expect(aspnet.text()).toContain('gpt-4.1-mini')

    expect(java.text()).toContain('HttpClient')
    expect(java.text()).toContain('HttpRequest')
    expect(java.text()).toContain('/chat/completions')
    expect(java.text()).toContain('gpt-4.1-mini')

    expect(powershell.text()).toContain('Invoke-RestMethod')
    expect(powershell.text()).toContain('/chat/completions')
    expect(powershell.text()).toContain('Authorization')
    expect(powershell.text()).toContain('gpt-4.1-mini')

    expect(lua.text()).toContain('/chat/completions')
    expect(lua.text()).toContain('Authorization')
    expect(lua.text()).toContain('Content-Type')
    expect(lua.text()).toContain('gpt-4.1-mini')
  })

  it('renders agent documentation from components', () => {
    const cline = mount(HelpSetupClineSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })
    const continueDocs = mount(HelpSetupContinueSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })

    expect(cline.text()).toContain('cline auth')
    expect(cline.text()).toContain('https://proxy.example.com/openai-main/v1')
    expect(cline.text()).toContain('YOUR_PROMPTGATE_TOKEN')
    expect(cline.text()).toContain('gpt-4.1-mini')

    expect(continueDocs.text()).toContain('schema: v1')
    expect(continueDocs.text()).toContain('apiBase')
    expect(continueDocs.text()).toContain('YOUR_PROMPTGATE_TOKEN')
    expect(continueDocs.text()).toContain('~/.continue/config.yaml')
    expect(continueDocs.text()).toContain(
      '%USERPROFILE%\\.continue\\config.yaml',
    )
    expect(continueDocs.text()).toContain('"gpt-4.1-mini"')
    expect(continueDocs.text()).toContain('"gpt-4.1"')
  })

  it('renders OpenCode documentation from components', () => {
    const openClaw = mount(HelpSetupOpenClawSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })
    const openCode = mount(HelpSetupOpenCodeSnippet, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider: openaiProvider,
      },
    })

    expect(openClaw.text()).toContain('~/.openclaw/openclaw.json')
    expect(openClaw.text()).toContain('models')
    expect(openClaw.text()).toContain('providers')
    expect(openClaw.text()).toContain('openai-completions')
    expect(openClaw.text()).toContain(
      'https://proxy.example.com/openai-main/v1',
    )
    expect(openClaw.text()).toContain('YOUR_PROMPTGATE_TOKEN')
    expect(openClaw.text()).toContain('gpt-4.1-mini')
    expect(openClaw.text()).toContain('gpt-4.1')

    expect(openCode.text()).toContain('@ai-sdk/openai-compatible')
    expect(openCode.text()).toContain('~/.config/opencode/opencode.json')
    expect(openCode.text()).toContain('./opencode.json')
    expect(openCode.text()).toContain('"gpt-4.1-mini": {}')
    expect(openCode.text()).toContain('"gpt-4.1": {}')
  })
})
