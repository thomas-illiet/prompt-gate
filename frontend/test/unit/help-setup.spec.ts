import { describe, expect, it } from 'vitest'

import {
  availableSetupProviders,
  firstUsableModel,
  providerBaseUrl,
  providerHasModels,
  providerLabel,
} from '../../app/utils/help-setup'
import type { HelpSetupProvider } from '../../app/types/user-service'

const openaiProvider: HelpSetupProvider = {
  name: 'openai-main',
  displayName: 'OpenAI Main',
  type: 'openai',
  routePrefix: '/openai-main/v1',
  openaiBaseUrl: 'https://proxy.example.com/openai-main/v1',
  models: ['gpt-4.1-mini'],
}

const anthropicProvider: HelpSetupProvider = {
  name: 'anthropic-main',
  displayName: 'Anthropic Main',
  type: 'anthropic',
  routePrefix: '/anthropic-main',
  anthropicBaseUrl: 'https://proxy.example.com/anthropic-main',
  models: ['claude-3-7-sonnet-latest'],
}

describe('help setup utilities', () => {
  it('formats provider labels and base URLs', () => {
    expect(providerLabel(openaiProvider)).toBe('OpenAI Main')
    expect(providerBaseUrl(openaiProvider)).toBe(
      'https://proxy.example.com/openai-main/v1',
    )
    expect(providerBaseUrl(anthropicProvider)).toBe(
      'https://proxy.example.com/anthropic-main',
    )
  })

  it('uses a safe model placeholder when upstream models are unavailable', () => {
    expect(
      firstUsableModel({
        ...openaiProvider,
        models: [],
        modelsError: 'fetch models returned 502',
      }),
    ).toBe('MODEL_ID')
  })

  it('keeps only providers with available models', () => {
    const providerWithoutModels: HelpSetupProvider = {
      ...openaiProvider,
      name: 'openai-empty',
      models: [],
      modelsError: 'fetch models returned 502',
    }

    expect(providerHasModels(openaiProvider)).toBe(true)
    expect(providerHasModels(providerWithoutModels)).toBe(false)
    expect(
      availableSetupProviders([providerWithoutModels, openaiProvider]),
    ).toEqual([openaiProvider])
  })
})
