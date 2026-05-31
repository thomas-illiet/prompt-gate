import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import {
  toPromptHistoryErrorMessage,
  usePromptHistory,
} from '../../app/composables/usePromptHistory'
import type {
  PromptHistoryItem,
  PromptHistoryResponse,
} from '../../app/types/user-service'

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

const prompt: PromptHistoryItem = {
  id: 'prompt-id',
  interceptionId: 'interception-id',
  providerResponseId: 'response-id',
  provider: 'openai',
  providerType: 'openai',
  model: 'gpt-5',
  prompt: 'Alpha prompt',
  inputTokens: 10,
  outputTokens: 20,
  totalTokens: 30,
  durationMs: 1250,
  createdAt: '2026-01-01T00:00:00Z',
}

function response(
  items: PromptHistoryItem[],
  total = items.length,
): PromptHistoryResponse {
  return {
    items,
    page: 1,
    pageSize: 10,
    total,
  }
}

describe('usePromptHistory', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  it('loads prompt history and supports reload', async () => {
    apiFetch
      .mockResolvedValueOnce(response([prompt]))
      .mockResolvedValueOnce(response([]))

    const promptHistory = usePromptHistory()
    await vi.waitFor(() => expect(promptHistory.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/me/prompts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc',
    )
    expect(promptHistory.prompts.value).toEqual([prompt])
    expect(promptHistory.total.value).toBe(1)

    await promptHistory.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/me/prompts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc',
    )
  })

  it('updates query string for search and pagination', async () => {
    apiFetch
      .mockResolvedValueOnce(response([]))
      .mockResolvedValueOnce(response([prompt], 1))
      .mockResolvedValueOnce(response([], 20))
      .mockResolvedValueOnce(response([], 20))

    const promptHistory = usePromptHistory()
    await vi.waitFor(() => expect(promptHistory.loading.value).toBe(false))

    promptHistory.setSearch('alpha')
    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenNthCalledWith(
        2,
        '/api/v1/me/prompts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc&search=alpha',
      ),
    )

    promptHistory.setPage(2)
    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenNthCalledWith(
        3,
        '/api/v1/me/prompts?page=2&pageSize=10&sortBy=createdAt&sortDir=desc&search=alpha',
      ),
    )

    promptHistory.setPageSize(25)
    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenNthCalledWith(
        4,
        '/api/v1/me/prompts?page=1&pageSize=25&sortBy=createdAt&sortDir=desc&search=alpha',
      ),
    )
  })

  it('maps API errors to readable messages', () => {
    expect(toPromptHistoryErrorMessage(apiError('invalid_pagination'))).toBe(
      'Prompt history pagination is invalid.',
    )
  })
})
