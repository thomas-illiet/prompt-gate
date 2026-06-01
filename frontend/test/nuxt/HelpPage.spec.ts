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
        AppPageHero: true,
        AppSectionCard: {
          template: '<section><slot /></section>',
        },
        HelpSetupConfigurationCard: {
          template: '<div data-test="configuration-card" />',
        },
        HelpSetupDocumentationPanel: {
          template: '<div data-test="documentation-panel" />',
        },
        HelpSetupOperationalNotes: {
          template: '<div data-test="operational-notes" />',
        },
        HelpSetupProviderLoadingCard: true,
        VAlert: { template: '<div><slot /></div>' },
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
      }),
    })

    const wrapper = mountPage()

    expect(wrapper.get('[data-test="empty-state"]').text()).toContain(
      'No accessible setup provider yet',
    )
    expect(wrapper.get('[data-test="empty-state"]').text()).toContain(
      'groups grant access',
    )
  })

  it('shows setup cards for Anthropic providers without models', () => {
    useHelpSetupMock.mockReturnValue({
      error: shallowRef(null),
      loading: shallowRef(false),
      reload: vi.fn(),
      setup: shallowRef({
        proxyBaseUrl: 'https://proxy.example.com',
        providers: [anthropicProvider],
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
  })
})
