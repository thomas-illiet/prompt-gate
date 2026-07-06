import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import {
  toUserTokenErrorMessage,
  useUserTokens,
} from '../../app/composables/useUserTokens'
import type {
  CreatedUserToken,
  UserToken,
  UserTokenListResponse,
  UserTokenPayload,
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

const token: UserToken = {
  id: 'token-id',
  userId: 'user-id',
  name: 'personal_cli',
  description: 'CLI access',
  expiresAt: '2099-12-31T00:00:00Z',
  createdAt: '2026-01-01T00:00:00Z',
}

const payload: UserTokenPayload = {
  name: 'personal_cli',
  description: 'CLI access',
  expiresInDays: 30,
}

const createdToken: CreatedUserToken = {
  token: 'raw.jwt.value',
  tokenInfo: token,
}

function response(
  items: UserToken[],
  total = items.length,
): UserTokenListResponse {
  return {
    items,
    page: 1,
    pageSize: 10,
    total,
  }
}

describe('useUserTokens', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  it('loads tokens on creation and supports reload', async () => {
    apiFetch
      .mockResolvedValueOnce(response([token]))
      .mockResolvedValueOnce(response([]))

    const userTokens = useUserTokens()
    await vi.waitFor(() => expect(userTokens.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/tokens?page=1&pageSize=10&sortBy=createdAt&sortDir=desc&status=active',
    )
    expect(userTokens.tokens.value).toEqual([token])
    expect(userTokens.activeTokensCount.value).toBe(1)

    await userTokens.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/tokens?page=1&pageSize=10&sortBy=createdAt&sortDir=desc&status=active',
    )
    expect(userTokens.tokens.value).toEqual([])
  })

  it('creates, stores, and revokes user tokens', async () => {
    apiFetch
      .mockResolvedValueOnce(response([]))
      .mockResolvedValueOnce(createdToken)
      .mockResolvedValueOnce(response([token]))
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(response([]))

    const userTokens = useUserTokens()
    await vi.waitFor(() => expect(userTokens.loading.value).toBe(false))

    await userTokens.createToken(payload)
    await userTokens.revokeToken(token.id)

    expect(apiFetch).toHaveBeenNthCalledWith(2, '/api/v1/tokens', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    expect(userTokens.createdToken.value).toEqual(createdToken)
    expect(apiFetch).toHaveBeenNthCalledWith(4, `/api/v1/tokens/${token.id}`, {
      method: 'DELETE',
    })
  })

  it('filters tokens by search and status', async () => {
    const expiredToken: UserToken = {
      ...token,
      id: 'expired-id',
      name: 'expired_cli',
      expiresAt: '2020-01-01T00:00:00Z',
    }

    apiFetch
      .mockResolvedValueOnce(response([token]))
      .mockResolvedValueOnce(response([], 0))
      .mockResolvedValueOnce(response([expiredToken], 1))

    const userTokens = useUserTokens()
    await vi.waitFor(() => expect(userTokens.loading.value).toBe(false))
    expect(userTokens.statusFilter.value).toBe('active')

    expect(userTokens.tokenStats.value).toEqual({
      active: 1,
      all: 1,
      expired: 0,
      revoked: 0,
    })

    userTokens.setSearch('old')
    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenLastCalledWith(
        '/api/v1/tokens?page=1&pageSize=10&sortBy=createdAt&sortDir=desc&search=old&status=active',
      ),
    )

    userTokens.setSearch('')
    userTokens.setStatusFilter('expired')
    await vi.waitFor(() =>
      expect(apiFetch).toHaveBeenLastCalledWith(
        '/api/v1/tokens?page=1&pageSize=10&sortBy=createdAt&sortDir=desc&status=expired',
      ),
    )
  })

  it('maps API errors to readable messages', () => {
    expect(toUserTokenErrorMessage(apiError('invalid_token_name'))).toBe(
      'Virtual key name must use lowercase letters, numbers, dashes, or underscores.',
    )
    expect(toUserTokenErrorMessage(apiError('invalid_token_ttl'))).toBe(
      'Virtual key lifetime must be between 1 and 30 days.',
    )
  })
})
