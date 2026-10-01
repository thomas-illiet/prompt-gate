import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import {
  toAdminServiceAccountErrorMessage,
  useAdminServiceAccounts,
} from '../../app/composables/useAdminServiceAccounts'
import type {
  CreatedTokenResponse,
  ServiceAccount,
  ServiceAccountListResponse,
  ServiceAccountPayload,
  TokenListResponse,
  TokenPayload,
  TokenResponse,
} from '../../app/types/service-accounts'

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

const account: ServiceAccount = {
  id: 'service-account-id',
  identifier: 'ci_runner',
  name: 'CI runner',
  role: 'user',
  note: '',
  isActive: true,
  inputTokens: 1234,
  outputTokens: 5678,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const accountPayload: ServiceAccountPayload = {
  identifier: 'ci_runner',
  name: 'CI runner',
  isActive: true,
}

const token: TokenResponse = {
  id: 'token-id',
  userId: account.id,
  name: 'ci_token',
  description: 'CI access',
  expiresAt: '2026-12-31T00:00:00Z',
  createdAt: '2026-01-01T00:00:00Z',
}

const tokenPayload: TokenPayload = {
  name: 'ci_token',
  description: 'CI access',
  expiresInDays: 365,
}

const createdToken: CreatedTokenResponse = {
  token: 'raw.jwt.value',
  tokenInfo: token,
}

function accountResponse(
  items: ServiceAccount[],
  total = items.length,
): ServiceAccountListResponse {
  return {
    items,
    page: 1,
    pageSize: 10,
    total,
  }
}

function tokenResponse(
  items: TokenResponse[],
  total = items.length,
): TokenListResponse {
  return {
    items,
    page: 1,
    pageSize: 10,
    total,
  }
}

describe('useAdminServiceAccounts', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  it('loads service accounts on creation and supports reload', async () => {
    apiFetch
      .mockResolvedValueOnce(accountResponse([account]))
      .mockResolvedValueOnce(accountResponse([]))

    const adminServiceAccounts = useAdminServiceAccounts()
    await vi.waitFor(() =>
      expect(adminServiceAccounts.loading.value).toBe(false),
    )

    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/admin/service-accounts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc',
    )
    expect(adminServiceAccounts.accounts.value).toEqual([account])
    expect(adminServiceAccounts.activeAccountsCount.value).toBe(1)

    await adminServiceAccounts.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/service-accounts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc',
    )
    expect(adminServiceAccounts.accounts.value).toEqual([])
  })

  it('loads paged IP addresses for a service account', async () => {
    const addresses = [{ ip: '192.0.2.10', lastSeen: '2026-09-15T08:00:00Z' }]
    apiFetch
      .mockResolvedValueOnce(accountResponse([account]))
      .mockResolvedValueOnce({ items: addresses, page: 1, pageSize: 10, total: 1 })
    const adminServiceAccounts = useAdminServiceAccounts()
    await vi.waitFor(() => expect(adminServiceAccounts.loading.value).toBe(false))
    await adminServiceAccounts.loadIPAddresses(account.id)
    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      `/api/v1/admin/service-accounts/${account.id}/ips?page=1&pageSize=10&sortBy=lastSeen&sortDir=desc`,
    )
    expect(adminServiceAccounts.ipAddresses.value).toEqual(addresses)
  })

  it('creates, updates, loads, and deletes service accounts', async () => {
    apiFetch
      .mockResolvedValueOnce(accountResponse([]))
      .mockResolvedValueOnce(account)
      .mockResolvedValueOnce(accountResponse([account]))
      .mockResolvedValueOnce(account)
      .mockResolvedValueOnce(account)
      .mockResolvedValueOnce(accountResponse([account]))
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(accountResponse([]))

    const adminServiceAccounts = useAdminServiceAccounts()
    await vi.waitFor(() =>
      expect(adminServiceAccounts.loading.value).toBe(false),
    )

    await adminServiceAccounts.createAccount(accountPayload)
    await adminServiceAccounts.loadAccount(account.id)
    await adminServiceAccounts.updateAccount(account.id, accountPayload)
    await adminServiceAccounts.deleteAccount(account.id)

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/service-accounts',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(accountPayload),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      4,
      `/api/v1/admin/service-accounts/${account.id}`,
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      5,
      `/api/v1/admin/service-accounts/${account.id}`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(accountPayload),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      7,
      `/api/v1/admin/service-accounts/${account.id}`,
      { method: 'DELETE' },
    )
  })

  it('updates a service account note and reloads accounts', async () => {
    const notedAccount = { ...account, note: 'Owned by platform.' }
    apiFetch
      .mockResolvedValueOnce(accountResponse([account]))
      .mockResolvedValueOnce(notedAccount)
      .mockResolvedValueOnce(accountResponse([notedAccount]))

    const adminServiceAccounts = useAdminServiceAccounts()
    await vi.waitFor(() =>
      expect(adminServiceAccounts.loading.value).toBe(false),
    )

    const updated = await adminServiceAccounts.updateAccountNote(
      account.id,
      'Owned by platform.',
    )

    expect(updated).toEqual(notedAccount)
    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      `/api/v1/admin/service-accounts/${account.id}/note`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ note: 'Owned by platform.' }),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      '/api/v1/admin/service-accounts?page=1&pageSize=10&sortBy=createdAt&sortDir=desc',
    )
    expect(adminServiceAccounts.accounts.value).toEqual([notedAccount])
  })

  it('loads, creates, stores, and revokes service account tokens', async () => {
    apiFetch
      .mockResolvedValueOnce(accountResponse([]))
      .mockResolvedValueOnce(tokenResponse([token]))
      .mockResolvedValueOnce(tokenResponse([token]))
      .mockResolvedValueOnce(createdToken)
      .mockResolvedValueOnce(tokenResponse([token]))
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(tokenResponse([]))

    const adminServiceAccounts = useAdminServiceAccounts()
    await vi.waitFor(() =>
      expect(adminServiceAccounts.loading.value).toBe(false),
    )

    await adminServiceAccounts.loadTokens(account.id)
    await adminServiceAccounts.loadTokens(account.id, true)
    await adminServiceAccounts.createToken(account.id, tokenPayload)
    await adminServiceAccounts.revokeToken(account.id, token.id, true)

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      `/api/v1/admin/service-accounts/${account.id}/tokens?page=1&pageSize=10&sortBy=createdAt&sortDir=desc`,
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      `/api/v1/admin/service-accounts/${account.id}/tokens?page=1&pageSize=10&sortBy=createdAt&sortDir=desc&includeRevoked=true`,
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      4,
      `/api/v1/admin/service-accounts/${account.id}/tokens`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(tokenPayload),
      },
    )
    expect(adminServiceAccounts.createdToken.value).toEqual(createdToken)
    expect(apiFetch).toHaveBeenNthCalledWith(
      6,
      `/api/v1/admin/service-accounts/${account.id}/tokens/${token.id}`,
      { method: 'DELETE' },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      7,
      `/api/v1/admin/service-accounts/${account.id}/tokens?page=1&pageSize=10&sortBy=createdAt&sortDir=desc&includeRevoked=true`,
    )
  })

  it('maps API errors to readable messages', () => {
    expect(
      toAdminServiceAccountErrorMessage(apiError('service_account_not_found')),
    ).toBe('Service account no longer exists.')
    expect(
      toAdminServiceAccountErrorMessage(apiError('identifier_conflict')),
    ).toBe('Another service account already uses this identifier.')
    expect(
      toAdminServiceAccountErrorMessage(apiError('invalid_token_ttl')),
    ).toBe('Virtual key lifetime must be between 1 and 365 days.')
  })
})
