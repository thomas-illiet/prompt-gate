import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import {
  toAdminPromptHistoryErrorMessage,
  useAdminPromptHistory,
} from '../../app/composables/useAdminPromptHistory'
import type {
  AdminPromptHistoryItem,
  AdminPromptHistoryResponse,
} from '../../app/types/user-service'
import type { AdminUser, UserListResponse } from '../../app/types/users'

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

const prompt: AdminPromptHistoryItem = {
  id: 'prompt-id',
  interceptionId: 'interception-id',
  providerResponseId: 'response-id',
  provider: 'openai',
  providerType: 'openai',
  model: 'gpt-5',
  prompt: 'Alpha prompt',
  userId: 'user-id',
  userName: 'Prompt User',
  userEmail: 'prompt@example.com',
  userPreferredUsername: 'prompt-user',
  inputTokens: 10,
  outputTokens: 20,
  totalTokens: 30,
  durationMs: 1250,
  createdAt: '2026-01-01T00:00:00Z',
}

const user: AdminUser = {
  id: 'user-id',
  sub: 'sub',
  preferredUsername: 'prompt-user',
  email: 'prompt@example.com',
  name: 'Prompt User',
  role: 'user',
  note: '',
  isActive: true,
  inputTokens: 10,
  outputTokens: 20,
  expiresAt: null,
  lastLoginAt: '2026-01-01T00:00:00Z',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function promptResponse(
  items: AdminPromptHistoryItem[],
  total = items.length,
): AdminPromptHistoryResponse {
  return {
    items,
    page: 1,
    pageSize: 10,
    total,
  }
}

function userResponse(items: AdminUser[]): UserListResponse {
  return {
    items,
    page: 1,
    pageSize: 100,
    total: items.length,
  }
}

describe('useAdminPromptHistory', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  it('loads admin prompt history, user options, and supports reload', async () => {
    apiFetch
      .mockResolvedValueOnce(promptResponse([prompt]))
      .mockResolvedValueOnce(userResponse([user]))
      .mockResolvedValueOnce(promptResponse([]))

    const adminPromptHistory = useAdminPromptHistory()
    await vi.waitFor(() => expect(adminPromptHistory.loading.value).toBe(false))
    await vi.waitFor(() =>
      expect(adminPromptHistory.loadingUsers.value).toBe(false),
    )

    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/admin/prompts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc',
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/users?page=1&pageSize=100&sortBy=preferredUsername&sortDir=asc',
    )
    expect(adminPromptHistory.prompts.value).toEqual([prompt])
    expect(adminPromptHistory.users.value).toEqual([user])

    await adminPromptHistory.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      '/api/v1/admin/prompts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc',
    )
  })

  it('updates query string for user filter, search, and user option search', async () => {
    apiFetch
      .mockResolvedValueOnce(promptResponse([]))
      .mockResolvedValueOnce(userResponse([user]))
      .mockResolvedValueOnce(promptResponse([prompt]))
      .mockResolvedValueOnce(promptResponse([prompt]))
      .mockResolvedValueOnce(userResponse([user]))

    const adminPromptHistory = useAdminPromptHistory()
    await vi.waitFor(() => expect(adminPromptHistory.loading.value).toBe(false))
    await vi.waitFor(() =>
      expect(adminPromptHistory.loadingUsers.value).toBe(false),
    )

    adminPromptHistory.setUserId('user-id')
    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenNthCalledWith(
        3,
        '/api/v1/admin/prompts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc&userId=user-id',
      ),
    )

    adminPromptHistory.setSearch('alpha')
    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenNthCalledWith(
        4,
        '/api/v1/admin/prompts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc&search=alpha&userId=user-id',
      ),
    )

    await adminPromptHistory.setUserSearch('prompt')

    expect(apiFetch).toHaveBeenNthCalledWith(
      5,
      '/api/v1/admin/users?page=1&pageSize=100&sortBy=preferredUsername&sortDir=asc&search=prompt',
    )
  })

  it('maps API errors to readable messages', () => {
    expect(toAdminPromptHistoryErrorMessage(apiError('invalid_sort'))).toBe(
      'Selected prompt history sort is invalid.',
    )
  })
})
