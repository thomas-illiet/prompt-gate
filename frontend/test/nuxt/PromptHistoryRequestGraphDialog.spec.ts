import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import PromptHistoryRequestGraph from '../../app/components/PromptHistory/PromptHistoryRequestGraph.vue'
import PromptHistoryRequestGraphDialog from '../../app/components/PromptHistory/PromptHistoryRequestGraphDialog.vue'
import type {
  AdminPromptHistoryItem,
  PromptHistoryItem,
} from '../../app/types/user-service'

const prompt: PromptHistoryItem = {
  id: 'prompt-id',
  interceptionId: 'interception-id',
  providerResponseId: 'response-id',
  provider: 'openai',
  providerType: 'openai',
  model: 'gpt-5',
  prompt: 'Alpha prompt',
  inputTokens: 1200,
  outputTokens: 340,
  totalTokens: 1540,
  durationMs: null,
  createdAt: '2026-01-01T00:00:00Z',
}

const adminPrompt: AdminPromptHistoryItem = {
  ...prompt,
  id: 'admin-prompt-id',
  userId: 'user-id',
  userName: 'Ada',
  userEmail: 'ada@example.com',
  userPreferredUsername: 'ada',
  clientIp: '198.51.100.7',
}

function mountDialog(
  dialogPrompt: PromptHistoryItem | AdminPromptHistoryItem = prompt,
) {
  return mount(PromptHistoryRequestGraphDialog, {
    props: {
      actorLabel: 'You',
      modelValue: true,
      prompt: dialogPrompt,
    },
    global: {
      components: {
        PromptHistoryRequestGraph,
      },
      stubs: {
        AppDialogCard: {
          props: ['modelValue', 'subtitle', 'title'],
          template:
            '<section v-if="modelValue" data-test="dialog" :data-title="title" :data-subtitle="subtitle"><slot /><slot name="actions" /></section>',
        },
        AppDialogCloseButton: {
          emits: ['click'],
          template:
            '<button type="button" data-test="close" @click="$emit(\'click\')">Close</button>',
        },
        VSpacer: { template: '<span />' },
      },
    },
  })
}

function mountGraph() {
  return mount(PromptHistoryRequestGraph, {
    props: {
      actorLabel: 'You',
      prompt,
    },
  })
}

function mockSvgCoordinates(svg: SVGSVGElement) {
  Object.assign(svg, {
    createSVGPoint: () => {
      const point = {
        x: 0,
        y: 0,
        matrixTransform: () => ({ x: point.x, y: point.y }),
      }
      return point
    },
    getScreenCTM: () => ({
      inverse: () => ({}),
    }),
  })
}

describe('PromptHistoryRequestGraphDialog', () => {
  it('renders request metrics and pending duration fallback', () => {
    const wrapper = mountDialog()

    expect(wrapper.get('[data-test="dialog"]').attributes('data-title')).toBe(
      'Request graph',
    )
    expect(
      wrapper.get('[data-test="dialog"]').attributes('data-subtitle'),
    ).toBe('openai / gpt-5')

    const text = wrapper.text()
    expect(text).toContain('You')
    expect(text).toContain('openai')
    expect(text).toContain('gpt-5')
    expect(text).toContain('Pending')
    expect(text).toContain('Alpha prompt')
    expect(text).not.toContain('Client IP')
    expect(wrapper.get('svg').attributes('viewBox')).toBe('0 0 1100 386')
    expect(
      wrapper.findAll('.prompt-history-request-graph__edge-label-box'),
    ).toHaveLength(4)
    const firstNodeBox = wrapper
      .findAll('.prompt-history-request-graph__node')[0]
      ?.get('rect')
    const firstLabelBox = wrapper.findAll(
      '.prompt-history-request-graph__edge-label-box',
    )[0]
    if (!firstNodeBox || !firstLabelBox) {
      throw new Error('expected graph nodes and edge labels')
    }
    expect(Number(firstLabelBox.attributes('y'))).toBeGreaterThan(
      Number(firstNodeBox.attributes('y')),
    )
    expect(Number(firstLabelBox.attributes('y'))).toBeLessThan(
      Number(firstNodeBox.attributes('y')) + 78,
    )
    const nodeBoxes = wrapper
      .findAll('.prompt-history-request-graph__node')
      .map((node) => node.get('rect'))
    const actorBox = nodeBoxes[0]
    const providerBox = nodeBoxes[2]
    const modelBox = nodeBoxes[3]
    const responseBox = nodeBoxes[4]
    if (!actorBox || !providerBox || !modelBox || !responseBox) {
      throw new Error('expected graph node boxes')
    }
    expect(modelBox.attributes('x')).toBe(actorBox.attributes('x'))
    expect(responseBox.attributes('x')).toBe(providerBox.attributes('x'))
    const inputLabelBox = wrapper.findAll(
      '.prompt-history-request-graph__edge-label-box',
    )[1]
    const outputLabelBox = wrapper.findAll(
      '.prompt-history-request-graph__edge-label-box',
    )[3]
    if (!inputLabelBox || !outputLabelBox) {
      throw new Error('expected token edge labels')
    }
    expect(Number(inputLabelBox.attributes('y'))).toBeGreaterThan(
      Number(firstNodeBox.attributes('y')),
    )
    expect(Number(inputLabelBox.attributes('y'))).toBeLessThan(
      Number(firstNodeBox.attributes('y')) + 78,
    )
    expect(Number(outputLabelBox.attributes('x'))).toBeGreaterThan(
      Number(modelBox.attributes('x')) + 160,
    )
    expect(Number(outputLabelBox.attributes('y'))).toBeGreaterThan(
      Number(modelBox.attributes('y')),
    )
    expect(Number(outputLabelBox.attributes('y'))).toBeLessThan(
      Number(modelBox.attributes('y')) + 78,
    )
    const edges = wrapper.findAll('.prompt-history-request-graph__edge')
    expect(edges).toHaveLength(4)
    for (const edge of edges) {
      expect(edge.attributes('fill')).toBe('none')
    }
  })

  it('renders admin client IP in the graph and summary', () => {
    const wrapper = mountDialog(adminPrompt)

    const text = wrapper.text()
    expect(text).toContain('Client IP')
    expect(text).toContain('198.51.100.7')
    expect(text).toContain('IP 198.51.100.7')
  })

  it('renders admin client IP fallback for older prompt rows', () => {
    const wrapper = mountDialog({
      ...adminPrompt,
      clientIp: '',
    })

    const text = wrapper.text()
    expect(text).toContain('Client IP')
    expect(text).toContain('IP unknown')
  })

  it('lets graph modules be dragged with pointer input', async () => {
    const wrapper = mountGraph()
    const svg = wrapper.get('svg').element as SVGSVGElement
    mockSvgCoordinates(svg)
    const firstNode = wrapper.findAll('.prompt-history-request-graph__node')[0]
    if (!firstNode) {
      throw new Error('expected a graph node')
    }

    expect(firstNode.get('rect').attributes('x')).toBe('48')

    await firstNode.trigger('pointerdown', {
      button: 0,
      clientX: 214,
      clientY: 81,
      pointerId: 1,
    })
    await wrapper.get('svg').trigger('pointermove', {
      clientX: 274,
      clientY: 111,
      pointerId: 1,
    })
    await wrapper.get('svg').trigger('pointerup', { pointerId: 1 })
    const movedFirstNode = wrapper.findAll(
      '.prompt-history-request-graph__node',
    )[0]
    if (!movedFirstNode) {
      throw new Error('expected a moved graph node')
    }

    expect(movedFirstNode.get('rect').attributes('x')).toBe('108')
  })
})
