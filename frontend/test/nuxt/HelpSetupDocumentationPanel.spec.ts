import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import HelpSetupDocumentationPanel from '../../app/components/HelpSetup/HelpSetupDocumentationPanel.vue'
import type { HelpSetupProvider } from '../../app/types/user-service'
import type { SetupGuide } from '../../app/types/setup-guides'

const provider: HelpSetupProvider = {
  name: 'openai-main',
  displayName: 'OpenAI Main',
  type: 'openai',
  routePrefix: '/openai-main/v1',
  openaiBaseUrl: 'https://proxy.example.com/openai-main/v1',
  models: ['gpt-4.1-mini'],
}
const guide = (
  id: string,
  title: string,
  position: number,
  template = '{{baseUrl}} {{model}}',
): SetupGuide => ({
  id,
  identifier: id,
  title,
  subtitle: `${title} setup`,
  icon: 'mdi-code-tags',
  compatibility: 'openai',
  modelMode: 'single',
  filePaths: [],
  template,
  enabled: true,
  position,
  createdAt: '',
  updatedAt: '',
})
const global = {
  stubs: {
    HelpSetupSnippetCard: {
      props: ['code', 'filePaths', 'subtitle', 'title'],
      template: '<section><h2>{{ title }}</h2><pre>{{ code }}</pre></section>',
    },
    VList: { template: '<nav><slot /></nav>' },
    VListItem: {
      props: ['active', 'title'],
      emits: ['click'],
      template: '<button @click="$emit(\'click\')">{{ title }}</button>',
    },
    VSelect: { template: '<div />' },
  },
}

describe('HelpSetupDocumentationPanel', () => {
  it('renders database guides in their supplied order', () => {
    const wrapper = mount(HelpSetupDocumentationPanel, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider,
        guides: [guide('python', 'Python', 0), guide('curl', 'curl', 1)],
      },
    })
    expect(wrapper.findAll('button').map((item) => item.text())).toEqual([
      'Python',
      'curl',
    ])
    expect(wrapper.text()).toContain(
      'https://proxy.example.com/openai-main/v1 gpt-4.1-mini',
    )
  })
  it('switches to another guide and renders its template', async () => {
    const wrapper = mount(HelpSetupDocumentationPanel, {
      global,
      props: {
        model: 'gpt-4.1-mini',
        provider,
        guides: [
          guide('first', 'First', 0),
          guide('second', 'Second', 1, 'provider={{providerName}}'),
        ],
      },
    })
    await wrapper.findAll('button')[1]!.trigger('click')
    expect(wrapper.text()).toContain('provider=openai-main')
  })
  it('renders nothing when no compatible guide is supplied', () => {
    const wrapper = mount(HelpSetupDocumentationPanel, {
      global,
      props: { model: 'gpt-4.1-mini', provider, guides: [] },
    })
    expect(wrapper.text()).toBe('')
  })
})
