import { describe, expect, it } from 'vitest'

import type {
  ModelLabProvider,
  ModelLabTarget,
  ModelLabTurn,
} from '../../app/types/model-lab'
import {
  flattenModelLabCatalog,
  modelLabPromptByteLength,
  modelLabTargetKey,
} from '../../app/utils/model-lab'
import { buildModelLabPayload } from '../../app/utils/model-lab-payload'
import { buildModelLabContextWindow } from '../../app/utils/model-lab-context'

const target: ModelLabTarget = {
  model: 'shared-name',
  providerDisplayName: 'OpenAI Main',
  providerName: 'openai-main',
  providerType: 'openai',
  protocol: 'openai-chat',
}

describe('Model Lab utilities', () => {
  it('keeps same-named models distinct by provider', () => {
    expect(modelLabTargetKey(target)).not.toBe(
      modelLabTargetKey({ ...target, providerName: 'openai-backup' }),
    )
  })

  it('returns only runnable catalog entries', () => {
    const providers: ModelLabProvider[] = [
      {
        catalog: { fetchedAt: null, state: 'fresh' },
        displayName: 'OpenAI Main',
        name: 'openai-main',
        type: 'openai',
        models: [
          {
            displayName: null,
            id: 'runnable',
            playground: { protocol: 'openai-chat', supported: true },
            pricing: null,
          },
          {
            displayName: null,
            id: 'metadata-only',
            playground: { protocol: 'openai-chat', supported: false },
            pricing: null,
          },
        ],
      },
    ]

    expect(flattenModelLabCatalog(providers).map((item) => item.model)).toEqual([
      'runnable',
    ])
  })

  it('builds a multi-turn text payload without tool fields', () => {
    const payload = buildModelLabPayload(target, [
      { content: 'hello', role: 'user' },
      { content: 'hi', role: 'assistant' },
      { content: 'continue', role: 'user' },
    ], {
      maxOutputTokens: 512,
      temperature: 0.2,
    })

    expect(payload).toEqual({
      max_tokens: 512,
      messages: [
        { content: 'hello', role: 'user' },
        { content: 'hi', role: 'assistant' },
        { content: 'continue', role: 'user' },
      ],
      model: 'shared-name',
      stream: true,
      temperature: 0.2,
    })
    expect(payload).not.toHaveProperty('tools')
    expect(payload.messages).toHaveLength(3)
  })

  it('keeps the newest complete turns that fit every target envelope', () => {
    const completedTurn = (
      id: string,
      prompt: string,
      output: string,
    ): ModelLabTurn => ({
      comparisons: [],
      context: [{ content: prompt, role: 'user' }],
      id,
      omittedTurns: 0,
      primary: {
        clientDurationMs: 10,
        clientTtftMs: 2,
        error: null,
        id: `${id}-lane`,
        output,
        status: 'completed',
        target,
        usage: { inputTokens: 1, outputTokens: 1 },
      },
      prompt,
      settings: { maxOutputTokens: 32, temperature: 0.2 },
    })
    const turns = [
      completedTurn('old', 'old '.repeat(20), 'answer '.repeat(20)),
      completedTurn('new', 'new', 'recent answer'),
    ]
    const settings = { maxOutputTokens: 32, temperature: 0.2 }
    const onlyRecent = buildModelLabContextWindow({
      maxBytes: 260,
      prompt: 'next',
      settings,
      targets: [target, { ...target, model: 'a-much-longer-model-name' }],
      turns,
    })

    expect(onlyRecent.fits).toBe(true)
    expect(onlyRecent.omittedTurns).toBe(1)
    expect(onlyRecent.messages).toEqual([
      { content: 'new', role: 'user' },
      { content: 'recent answer', role: 'assistant' },
      { content: 'next', role: 'user' },
    ])
  })

  it('never truncates a current message that cannot fit on its own', () => {
    const window = buildModelLabContextWindow({
      maxBytes: 64,
      prompt: '🙂'.repeat(100),
      settings: { maxOutputTokens: 32, temperature: 0.2 },
      targets: [target],
      turns: [],
    })

    expect(window.fits).toBe(false)
    expect(window.messages).toEqual([
      { content: '🙂'.repeat(100), role: 'user' },
    ])
  })

  it('measures the UTF-8 prompt size instead of assuming one byte per character', () => {
    expect(modelLabPromptByteLength('hello')).toBe(5)
    expect(modelLabPromptByteLength('🙂')).toBe(4)
  })
})
