import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import HelpPage from '../../app/pages/help.vue'
import type { HelpSetupProvider } from '../../app/types/user-service'

const { useHelpSetupMock, useHelpSnippetSelectionMock } = vi.hoisted(() => {
  return {
    useHelpSetupMock: vi.fn(),
    useHelpSnippetSelectionMock: vi.fn(() => ({
      modelOptions: { value: [] as string[] },
      selectedModel: { value: '' },
      selectedProvider: { value: null as HelpSetupProvider | null },
      selectedProviderName: { value: '' },
    })),
  }
})

mockNuxtImport('useHelpSetup', () => useHelpSetupMock)
mockNuxtImport('useHelpSnippetSelection', () => useHelpSnippetSelectionMock)

const anthropicProvider: HelpSetupProvider = {
  name: 'anthropic-main',
  displayName: 'Anthropic Main',
  type: 'anthropic',
  routePrefix: '/anthropic-main',
  anthropicBaseUrl: 'https://proxy.example.com/anthropic-main',
  models: [],
}

function mountPage() {
  return mount(HelpPage, {
    global: {
      stubs: {
        AppEmptyState: {
          props: ['text', 'title'],
          template: '<div data-test="empty-state">{{ title }} {{ text }}</div>',
        },
        HelpSetupHero: {
          props: ['loading', 'providerCount'],
          template:
            '<div data-test="setup-hero"><span data-test="hero-provider-count">{{ providerCount }}</span><span data-test="hero-loading">{{ loading }}</span></div>',
        },
        AppSectionCard: {
          template: '<section><slot /></section>',
        },
        HelpSetupConfigurationCard: {
          template: '<div data-test="configuration-card" />',
        },
        HelpSetupDocumentationPanel: {
          template: '<div data-test="documentation-panel" />',
        },
        HelpSetupProviderLoadingCard: true,
        VAlert: { template: '<div><slot /></div>' },
        VBtn: { template: '<button><slot /></button>' },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<div><slot /></div>' },
        VRow: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('HelpPage', () => {
  beforeEach(() => {
    useHelpSetupMock.mockReset()
    useHelpSnippetSelectionMock.mockClear()
  })

  it('explains empty setup as missing accessible setup providers', () => {
    useHelpSetupMock.mockReturnValue({
      error: shallowRef(null),
      loading: shallowRef(false),
      reload: vi.fn(),
      setup: shallowRef({
        proxyBaseUrl: 'https://proxy.example.com',
        providers: [],
        guides: [],
      }),
    })

    const wrapper = mountPage()

    expect(wrapper.get('[data-test="empty-state"]').text()).toContain(
      'The launchpad is waiting for a route',
    )
    expect(wrapper.get('[data-test="empty-state"]').text()).toContain(
      'groups grant access',
    )
    expect(wrapper.get('[data-test="hero-provider-count"]').text()).toBe('0')
    expect(wrapper.get('[data-test="hero-loading"]').text()).toBe('false')
  })

  it('shows setup cards for Anthropic providers without models', () => {
    useHelpSetupMock.mockReturnValue({
      error: shallowRef(null),
      loading: shallowRef(false),
      reload: vi.fn(),
      setup: shallowRef({
        proxyBaseUrl: 'https://proxy.example.com',
        providers: [anthropicProvider],
        guides: [
          {
            id: 'claude',
            identifier: 'claude-code',
            title: 'Claude Code',
            subtitle: 'Claude setup',
            icon: 'mdi-code-tags',
            compatibility: 'anthropic',
            modelMode: 'none',
            syntaxLanguage: 'bash',
            filePaths: [],
            template: '{{anthropicBaseUrl}}',
            enabled: true,
            position: 0,
            createdAt: '',
            updatedAt: '',
          },
        ],
      }),
    })
    useHelpSnippetSelectionMock.mockReturnValueOnce({
      modelOptions: shallowRef(['No model required']),
      selectedModel: shallowRef('No model required'),
      selectedProvider: shallowRef(anthropicProvider),
      selectedProviderName: shallowRef('anthropic-main'),
    })

    const wrapper = mountPage()

    expect(wrapper.find('[data-test="empty-state"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="configuration-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="documentation-panel"]').exists()).toBe(
      true,
    )
    expect(wrapper.text()).not.toContain('Before running snippets')
    expect(wrapper.get('[data-test="hero-provider-count"]').text()).toBe('1')
    expect(wrapper.get('[data-test="hero-loading"]').text()).toBe('false')
  })

  it('shows an indeterminate provider count and retries loading failures', async () => {
    const reload = vi.fn()
    useHelpSetupMock.mockReturnValue({
      error: shallowRef('Setup routes are unavailable.'),
      loading: shallowRef(true),
      reload,
      setup: shallowRef(null),
    })

    const wrapper = mountPage()

    expect(wrapper.get('[data-test="hero-provider-count"]').text()).toBe('0')
    expect(wrapper.get('[data-test="hero-loading"]').text()).toBe('true')
    await wrapper.get('[data-test="help-setup-retry"]').trigger('click')
    expect(reload).toHaveBeenCalledOnce()
  })
})
