import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { nextTick, shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ModelsPage from '../../app/pages/models.vue'
import type { ModelLabCatalogResponse } from '../../app/types/model-lab'

const { useModelLabCatalogMock, useModelLabChatMock } = vi.hoisted(() => ({
  useModelLabCatalogMock: vi.fn(),
  useModelLabChatMock: vi.fn(),
}))

mockNuxtImport('useModelLabCatalog', () => useModelLabCatalogMock)
mockNuxtImport('useModelLabChat', () => useModelLabChatMock)

function createCatalog(): ModelLabCatalogResponse {
  return {
    enabled: true,
    generatedAt: '2026-10-25T08:00:00Z',
    limits: {
      maxOutputTokens: 4096,
      maxPromptBytes: 32_768,
      maxTargets: 3,
    },
    providers: [
      {
        catalog: {
          fetchedAt: '2026-10-25T08:00:00Z',
          state: 'fresh',
        },
        displayName: 'OpenAI',
        models: [
          {
            displayName: 'GPT Test',
            id: 'gpt-test',
            playground: { protocol: 'openai-chat', supported: true },
            pricing: null,
          },
          {
            displayName: 'Unsupported',
            id: 'unsupported',
            playground: { protocol: 'openai-chat', supported: false },
            pricing: null,
          },
        ],
        name: 'openai',
        type: 'openai',
      },
      {
        catalog: {
          fetchedAt: '2026-10-25T08:00:00Z',
          state: 'fresh',
        },
        displayName: 'Anthropic',
        models: [
          {
            displayName: 'Claude Test',
            id: 'claude-test',
            playground: { protocol: 'anthropic-messages', supported: true },
            pricing: null,
          },
        ],
        name: 'anthropic',
        type: 'anthropic',
      },
    ],
  }
}

function createChatState() {
  return {
    cancelLane: vi.fn(),
    draft: shallowRef(''),
    draftError: shallowRef(''),
    error: shallowRef<string | null>(null),
    maxAllowedOutputTokens: shallowRef(4096),
    maxOutputTokens: shallowRef(1024),
    maxPromptLength: shallowRef(32_768),
    maxSelectedTargets: shallowRef(2),
    reset: vi.fn(),
    retryLane: vi.fn(),
    running: shallowRef(false),
    selectedTargets: shallowRef([]),
    send: vi.fn(),
    setDraft: vi.fn(),
    setMaxOutputTokens: vi.fn(),
    setSelectedTargets: vi.fn(),
    setTemperature: vi.fn(),
    stopAll: vi.fn(),
    temperature: shallowRef(0.7),
    turns: shallowRef([]),
  }
}

function mountPage() {
  return mount(ModelsPage, {
    global: {
      stubs: {
        ModelLabChatPanel: {
          props: ['modelsLoading', 'targets'],
          template: `
            <section
              data-test="model-lab-panel"
              :data-models-loading="modelsLoading"
            >
              {{ targets.length }}
            </section>
          `,
        },
        VContainer: { template: '<main><slot /></main>' },
      },
    },
  })
}

describe('ModelsPage', () => {
  beforeEach(() => {
    useModelLabCatalogMock.mockReset()
    useModelLabChatMock.mockReset()
    useModelLabChatMock.mockReturnValue(createChatState())
  })

  it('passes the live runnable model catalog to the unified chat panel', async () => {
    const catalog = shallowRef<ModelLabCatalogResponse | null>(createCatalog())
    useModelLabCatalogMock.mockReturnValue({
      catalog,
      error: shallowRef<string | null>(null),
      loading: shallowRef(false),
      reload: vi.fn(),
    })

    const wrapper = mountPage()
    const panel = wrapper.get('[data-test="model-lab-panel"]')

    expect(panel.attributes('data-models-loading')).toBe('false')
    expect(panel.text()).toBe('2')
    expect(wrapper.find('[data-test="models-header"]').exists()).toBe(false)

    catalog.value = {
      ...catalog.value!,
      providers: catalog.value!.providers.slice(0, 1),
    }
    await nextTick()

    expect(wrapper.get('[data-test="model-lab-panel"]').text()).toBe('1')
  })

  it('passes the model loading state to the unified chat panel', () => {
    useModelLabCatalogMock.mockReturnValue({
      catalog: shallowRef<ModelLabCatalogResponse | null>(null),
      error: shallowRef<string | null>(null),
      loading: shallowRef(true),
      reload: vi.fn(),
    })

    const wrapper = mountPage()

    expect(
      wrapper
        .get('[data-test="model-lab-panel"]')
        .attributes('data-models-loading'),
    ).toBe('true')
  })
})
