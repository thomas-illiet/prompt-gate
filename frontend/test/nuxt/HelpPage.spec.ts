import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import HelpPage from '../../app/pages/help.vue'

const { useHelpSetupMock, useHelpSnippetSelectionMock } = vi.hoisted(() => {
  return {
    useHelpSetupMock: vi.fn(),
    useHelpSnippetSelectionMock: vi.fn(() => ({
      modelOptions: { value: [] },
      selectedModel: { value: '' },
      selectedProvider: { value: null },
      selectedProviderName: { value: '' },
    })),
  }
})

mockNuxtImport('useHelpSetup', () => useHelpSetupMock)
mockNuxtImport('useHelpSnippetSelection', () => useHelpSnippetSelectionMock)

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
        HelpSetupConfigurationCard: true,
        HelpSetupDocumentationPanel: true,
        HelpSetupOperationalNotes: true,
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

  it('explains empty setup as missing accessible providers with models', () => {
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
      'No accessible provider with models yet',
    )
    expect(wrapper.get('[data-test="empty-state"]').text()).toContain(
      'groups grant access',
    )
  })
})
