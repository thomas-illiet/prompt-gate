import { describe, expect, it } from 'vitest'

import {
  buildPromptHistoryGraphLayout,
  promptHistoryGraphEdgeLabelLeft,
  promptHistoryGraphNodeLeft,
} from '../../app/utils/prompt-history-graph'
import type {
  AdminPromptHistoryItem,
  PromptHistoryItem,
} from '../../app/types/user-service'

const prompt: PromptHistoryItem = {
  id: 'prompt/id',
  interceptionId: 'interception-id',
  providerResponseId: 'response-id',
  provider: 'openai',
  providerType: 'openai',
  model: 'gpt-5',
  prompt: 'Alpha prompt',
  inputTokens: 1200,
  outputTokens: 340,
  totalTokens: 1540,
  durationMs: 1250,
  createdAt: '2026-01-01T00:00:00Z',
}

const adminPrompt: AdminPromptHistoryItem = {
  ...prompt,
  id: 'admin-prompt-id',
  userId: 'user-id',
  userName: 'Ada',
  userEmail: 'ada@example.com',
  userPreferredUsername: 'ada',
  clientIp: '',
}

describe('prompt history graph layout', () => {
  it('builds stable default nodes, edges, and marker ids', () => {
    const layout = buildPromptHistoryGraphLayout({
      actorLabel: 'Ada Lovelace',
      prompt,
    })

    expect(layout.markerId).toBe('prompt-request-arrow-prompt-id')
    expect(layout.graphHeight).toBe(386)
    expect(layout.nodes.map((node) => node.key)).toEqual([
      'actor',
      'gateway',
      'provider',
      'model',
      'response',
    ])
    expect(layout.edges.map((edge) => edge.label)).toEqual([
      'request',
      '1\u202f200 input',
      'route',
      '340 output',
    ])
    expect(promptHistoryGraphNodeLeft(layout.nodes[0]!)).toBe(48)
    expect(promptHistoryGraphEdgeLabelLeft(layout.edges[0]!)).toBeGreaterThan(0)
  })

  it('uses admin IP fallback and caller-provided node positions', () => {
    const layout = buildPromptHistoryGraphLayout({
      actorLabel: '',
      nodePositions: { actor: { x: 260, y: 120 } },
      prompt: adminPrompt,
    })

    expect(layout.nodes[0]).toMatchObject({
      key: 'actor',
      title: 'You',
      x: 260,
      y: 120,
    })
    expect(layout.edges[0]?.label).toBe('IP unknown')
  })
})
