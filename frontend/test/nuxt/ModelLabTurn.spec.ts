import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ModelLabTurn from '../../app/components/ModelLab/ModelLabTurn.vue'
import type {
  ModelLabLane,
  ModelLabTarget,
  ModelLabTurn as ChatTurn,
} from '../../app/types/model-lab'

const primaryTarget: ModelLabTarget = {
  model: 'gpt-test',
  providerDisplayName: 'OpenAI Test',
  providerName: 'openai-test',
  providerType: 'openai',
  protocol: 'openai-chat',
}

const additionalTarget: ModelLabTarget = {
  model: 'claude-test',
  providerDisplayName: 'Anthropic Test',
  providerName: 'anthropic-test',
  providerType: 'anthropic',
  protocol: 'anthropic-messages',
}

function completedLane(
  target: ModelLabTarget,
  id = `lane-${target.model}`,
): ModelLabLane {
  return {
    clientDurationMs: 42,
    clientTtftMs: 12,
    error: null,
    id,
    output: `Answer from ${target.model}`,
    status: 'completed',
    target,
    usage: { inputTokens: 2, outputTokens: 3 },
  }
}

function turn(comparisons: ModelLabLane[] = []): ChatTurn {
  return {
    comparisons,
    context: [{ content: 'Hello', role: 'user' }],
    id: 'turn-1',
    omittedTurns: 0,
    primary: completedLane(primaryTarget),
    prompt: 'Hello',
    settings: { maxOutputTokens: 1024, temperature: 0.7 },
  }
}

function mountTurn(comparisons: ModelLabLane[] = []) {
  return mount(ModelLabTurn, {
    props: {
      turn: turn(comparisons),
      turnNumber: 1,
    },
    global: {
      stubs: {
        ModelLabChatResponse: {
          template: '<div data-test="primary-response">Primary response</div>',
        },
        ModelLabTurnComparison: {
          props: ['comparisons'],
          template:
            '<div v-if="comparisons.length" data-test="comparison">Response comparison</div>',
        },
        VAlert: true,
        VAvatar: true,
        VIcon: true,
      },
    },
  })
}

describe('ModelLabTurn', () => {
  it('renders one focused response when one model was selected', () => {
    const wrapper = mountTurn()

    expect(wrapper.find('[data-test="primary-response"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="comparison"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Add model')
  })

  it('renders side-by-side comparison when two models were selected', () => {
    const wrapper = mountTurn([
      completedLane(additionalTarget, 'lane-additional'),
    ])

    expect(wrapper.find('[data-test="primary-response"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="comparison"]').exists()).toBe(true)
  })
})
