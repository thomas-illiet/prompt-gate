import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ModelLabChatPanel from '../../app/components/ModelLab/ModelLabChatPanel.vue'
import type {
  ModelLabLane,
  ModelLabTarget,
  ModelLabTurn,
} from '../../app/types/model-lab'

const target: ModelLabTarget = {
  model: 'gpt-test',
  providerDisplayName: 'OpenAI Test',
  providerName: 'openai-test',
  providerType: 'openai',
  protocol: 'openai-chat',
}
const focusComposer = vi.fn()

function lane(): ModelLabLane {
  return {
    clientDurationMs: 42,
    clientTtftMs: 12,
    error: null,
    id: 'lane-1',
    output: 'Hello',
    status: 'completed',
    target,
    usage: { inputTokens: 2, outputTokens: 3 },
  }
}

const turn: ModelLabTurn = {
  comparisons: [],
  context: [{ content: 'Hello', role: 'user' }],
  id: 'turn-1',
  omittedTurns: 0,
  primary: lane(),
  prompt: 'Hello',
  settings: { maxOutputTokens: 1024, temperature: 0.7 },
}

function mountPanel(
  options: {
    modelsError?: string | null
    modelsLoading?: boolean
    targets?: ModelLabTarget[]
    turns?: ModelLabTurn[]
  } = {},
) {
  return mount(ModelLabChatPanel, {
    props: {
      draft: '',
      draftError: 'Enter a message.',
      error: null,
      maxAllowedOutputTokens: 4096,
      maxOutputTokens: 1024,
      maxPromptLength: 8192,
      maxSelectedTargets: 2,
      modelsError: options.modelsError ?? null,
      modelsLoading: options.modelsLoading ?? false,
      running: false,
      selectedTargets: options.targets?.slice(0, 1) ?? [],
      targets: options.targets ?? [],
      temperature: 0.7,
      turns: options.turns ?? [],
    },
    global: {
      stubs: {
        AppConfirmDialog: {
          emits: ['confirm', 'update:modelValue'],
          props: ['modelValue'],
          template:
            '<div v-if="modelValue" data-test="confirm-dialog"><slot /><button data-test="confirm-new-chat" @click="$emit(\'confirm\')">Confirm</button></div>',
        },
        AppEmptyState: {
          props: ['title'],
          template: '<div data-test="empty-state">{{ title }}</div>',
        },
        AppSectionCard: { template: '<section><slot /></section>' },
        ModelLabChatComposer: {
          setup(
            _props: unknown,
            { expose }: { expose: (value: object) => void },
          ) {
            expose({ focus: focusComposer })
          },
          template: '<div />',
        },
        ModelLabChatHeader: {
          emits: ['newChat'],
          template:
            '<button data-test="new-chat" @click="$emit(\'newChat\')">New chat</button>',
        },
        ModelLabConversation: true,
        VAlert: { template: '<div data-test="models-alert"><slot /></div>' },
        VBtn: {
          emits: ['click'],
          template: '<button @click="$emit(\'click\')"><slot /></button>',
        },
        VProgressCircular: true,
      },
    },
  })
}

describe('ModelLabChatPanel', () => {
  beforeEach(() => focusComposer.mockReset())

  it('distinguishes model loading from an empty authorized list', () => {
    const wrapper = mountPanel({ modelsLoading: true })

    expect(wrapper.text()).toContain('Loading your available models')
    expect(wrapper.find('[data-test="empty-state"]').exists()).toBe(false)
  })

  it('shows a retryable model error instead of the no-access state', async () => {
    const wrapper = mountPanel({ modelsError: 'Models unavailable.' })

    expect(wrapper.get('[data-test="models-alert"]').text()).toContain(
      'Models unavailable.',
    )
    expect(wrapper.find('[data-test="empty-state"]').exists()).toBe(false)

    await wrapper.get('[data-test="models-alert"] button').trigger('click')
    expect(wrapper.emitted('retryModels')).toHaveLength(1)
  })

  it('confirms before clearing a non-empty conversation', async () => {
    const wrapper = mountPanel({ targets: [target], turns: [turn] })

    await wrapper.get('[data-test="new-chat"]').trigger('click')
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(true)
    expect(wrapper.emitted('newChat')).toBeUndefined()

    await wrapper.get('[data-test="confirm-new-chat"]').trigger('click')
    expect(wrapper.emitted('newChat')).toHaveLength(1)
  })

  it('restores composer focus when an alternative response finishes', async () => {
    const wrapper = mountPanel({ targets: [target], turns: [turn] })
    const activeComparison: ModelLabLane = {
      ...lane(),
      id: 'lane-2',
      output: '',
      status: 'streaming',
    }

    await wrapper.setProps({
      turns: [{ ...turn, comparisons: [activeComparison] }],
    })
    expect(focusComposer).not.toHaveBeenCalled()

    await wrapper.setProps({
      turns: [
        {
          ...turn,
          comparisons: [
            { ...activeComparison, output: 'Alternative', status: 'completed' },
          ],
        },
      ],
    })
    await wrapper.vm.$nextTick()

    expect(focusComposer).toHaveBeenCalledOnce()
  })
})
