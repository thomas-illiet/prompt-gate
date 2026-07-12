import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'

import {
  toHelpSetupErrorMessage,
  useHelpSetup,
} from '../../app/composables/useHelpSetup'
import type { HelpSetupResponse } from '../../app/types/user-service'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return {
    apiFetch,
    useApiFetchMock: vi.fn(() => apiFetch),
  }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

function apiError(code: string) {
  return Object.assign(Object.create(FetchError.prototype), {
    response: {
      _data: { error: code },
    },
  }) as FetchError
}

const setupResponse: HelpSetupResponse = {
  proxyBaseUrl: 'https://proxy.example.com',
  guides: [],
  providers: [
    {
      name: 'openai-main',
      displayName: 'OpenAI Main',
      type: 'openai',
      routePrefix: '/openai-main/v1',
      openaiBaseUrl: 'https://proxy.example.com/openai-main/v1',
      models: ['gpt-4.1-mini'],
    },
  ],
}

describe('useHelpSetup', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
  })

  it('loads setup data and supports reload', async () => {
    apiFetch
      .mockResolvedValueOnce(setupResponse)
      .mockResolvedValueOnce({ ...setupResponse, providers: [] })

    const helpSetup = useHelpSetup()
    await vi.waitFor(() => expect(helpSetup.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenNthCalledWith(1, '/api/v1/me/help/setup')
    expect(helpSetup.setup.value).toEqual(setupResponse)

    await helpSetup.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(2, '/api/v1/me/help/setup')
    expect(helpSetup.setup.value?.providers).toEqual([])
  })

  it('maps API errors to readable messages', () => {
    expect(
      toHelpSetupErrorMessage(apiError('provider service unavailable')),
    ).toBe('provider service unavailable')
  })
})
