import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import {
  toAdminUserErrorMessage,
  useAdminUsers,
} from '../../app/composables/useAdminUsers'
import type {
  UserToken,
  UserTokenListResponse,
} from '../../app/types/user-service'
import type { AccessGroup, GroupListResponse } from '../../app/types/groups'
import type { AdminUser, UserListResponse } from '../../app/types/users'

const { downloadUsersCsvMock } = vi.hoisted(() => ({
  downloadUsersCsvMock: vi.fn(),
}))

vi.mock('../../app/utils/users-csv', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../app/utils/users-csv')>()),
  downloadUsersCsv: downloadUsersCsvMock,
}))

const { apiFetch, routeQuery, useApiFetchMock, useRouteMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  const routeQuery: Record<string, string> = {}
  return {
    apiFetch,
    routeQuery,
    useApiFetchMock: vi.fn(() => apiFetch),
    useRouteMock: vi.fn(() => ({ fullPath: '/admin/users', query: routeQuery })),
  }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)
mockNuxtImport('useRoute', () => useRouteMock)

function apiError(code: string) {
  return Object.assign(Object.create(FetchError.prototype), {
    response: {
      _data: { error: code },
    },
  }) as FetchError
}

const user: AdminUser = {
  id: 'user-id',
  sub: 'oidc-sub',
  preferredUsername: 'ada',
  email: 'ada@example.com',
  name: 'Ada Lovelace',
  role: 'user',
  note: '',
  isActive: true,
  lastLoginAt: '2026-01-02T00:00:00Z',
  inputTokens: 123,
  outputTokens: 456,
  expiresAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const token: UserToken = {
  id: 'token-id',
  userId: user.id,
  name: 'personal_cli',
  description: 'CLI access',
  expiresAt: '2099-12-31T00:00:00Z',
  createdAt: '2026-01-01T00:00:00Z',
}

const group: AccessGroup = {
  id: 'group-id',
  name: 'engineering',
  displayName: 'Engineering',
  description: 'Engineering access',
  providers: [],
  modelPatterns: ['^gpt-5'],
  excludedModelPatterns: [],
  members: [],
  providerCount: 0,
  modelPatternCount: 1,
  memberCount: 0,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function userResponse(
  items: AdminUser[],
  total = items.length,
): UserListResponse {
  return {
    items,
    page: 1,
    pageSize: 10,
    total,
  }
}

function tokenResponse(
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

function groupResponse(
  items: AccessGroup[],
  total = items.length,
): GroupListResponse {
  return {
    items,
    page: 1,
    pageSize: 100,
    total,
  }
}

describe('useAdminUsers', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    useRouteMock.mockClear()
    for (const key of Object.keys(routeQuery)) Reflect.deleteProperty(routeQuery, key)
    downloadUsersCsvMock.mockReset()
    setActivePinia(createPinia())
  })

  it('initializes the list search from a quota-risk account link', async () => {
    routeQuery.search = '00000000-0000-0000-0000-000000000001'
    apiFetch.mockResolvedValueOnce(userResponse([user]))

    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))

    expect(adminUsers.search.value).toBe(routeQuery.search)
    expect(apiFetch.mock.calls[0]?.[0]).toContain(
      `search=${routeQuery.search}`,
    )
  })

  it('loads admin users on creation and supports reload', async () => {
    apiFetch
      .mockResolvedValueOnce(userResponse([user]))
      .mockResolvedValueOnce(userResponse([]))

    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/admin/users?page=1&pageSize=10&sortBy=lastLoginAt&sortDir=desc',
    )
    expect(adminUsers.users.value).toEqual([user])

    await adminUsers.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/users?page=1&pageSize=10&sortBy=lastLoginAt&sortDir=desc',
    )
    expect(adminUsers.users.value).toEqual([])
  })

  it('exports all users matching the active filters across pages', async () => {
    const secondUser = { ...user, id: 'user-id-2', email: 'grace@example.com' }
    apiFetch
      .mockResolvedValueOnce(userResponse([user]))
      .mockResolvedValueOnce(userResponse([user], 2))
      .mockResolvedValueOnce(userResponse([secondUser], 2))

    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))

    await adminUsers.exportUsers()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/users?page=1&pageSize=100&sortBy=lastLoginAt&sortDir=desc',
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      '/api/v1/admin/users?page=2&pageSize=100&sortBy=lastLoginAt&sortDir=desc',
    )
    expect(downloadUsersCsvMock).toHaveBeenCalledWith(
      [user, secondUser],
      expect.stringMatching(/^promptgate-users-\d{4}-\d{2}-\d{2}\.csv$/),
    )
    expect(adminUsers.exporting.value).toBe(false)
  })

  it('loads and revokes selected user tokens', async () => {
    apiFetch
      .mockResolvedValueOnce(userResponse([]))
      .mockResolvedValueOnce(tokenResponse([token]))
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(
        tokenResponse([{ ...token, revokedAt: '2026-02-01T00:00:00Z' }]),
      )

    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))

    adminUsers.setTokenPage(2)
    adminUsers.setTokenPageSize(25)
    adminUsers.setTokenSort('expiresAt', 'asc')

    await adminUsers.loadTokens(user.id)
    expect(adminUsers.tokens.value).toEqual([token])

    await adminUsers.revokeUserToken(user.id, token.id)

    const expectedListUrl =
      `/api/v1/admin/users/${user.id}/tokens?` +
      'page=1&pageSize=25&sortBy=expiresAt&sortDir=asc'

    expect(apiFetch).toHaveBeenNthCalledWith(2, expectedListUrl)
    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      `/api/v1/admin/users/${user.id}/tokens/${token.id}`,
      { method: 'DELETE' },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(4, expectedListUrl)
    expect(adminUsers.tokens.value[0]?.revokedAt).toBe('2026-02-01T00:00:00Z')
  })

  it('creates a selected user token and reloads token rows', async () => {
    const created = { token: 'raw-secret', tokenInfo: token }
    apiFetch
      .mockResolvedValueOnce(userResponse([]))
      .mockResolvedValueOnce(created)
      .mockResolvedValueOnce(tokenResponse([token]))

    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))

    await adminUsers.createUserToken(user.id, {
      name: 'personal_cli',
      description: 'CLI access',
      expiresInDays: 30,
    })

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      `/api/v1/admin/users/${user.id}/tokens`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: 'personal_cli',
          description: 'CLI access',
          expiresInDays: 30,
        }),
      },
    )
    expect(adminUsers.createdToken.value).toEqual(created)
    expect(adminUsers.tokens.value).toEqual([token])
  })

  it('loads paged IP addresses for a user', async () => {
    const addresses = [
      {
        ip: '2001:db8::1',
        lastSeen: '2026-09-15T08:00:00Z',
        lastDeniedReason: null,
        lastDeniedAt: null,
      },
    ]
    apiFetch.mockResolvedValueOnce(userResponse([user])).mockResolvedValueOnce({
      items: addresses,
      page: 1,
      pageSize: 10,
      total: 1,
    })
    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))
    await adminUsers.loadIPAddresses(user.id)
    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      `/api/v1/admin/users/${user.id}/ips?page=1&pageSize=10&sortBy=lastSeen&sortDir=desc`,
    )
    expect(adminUsers.ipAddresses.value).toEqual(addresses)
  })

  it('clears all IP addresses for a user', async () => {
    apiFetch
      .mockResolvedValueOnce(userResponse([user]))
      .mockResolvedValueOnce(undefined)
    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))
    adminUsers.ipAddresses.value = [
      {
        ip: '192.0.2.1',
        lastSeen: '2026-09-15T08:00:00Z',
        lastDeniedReason: null,
        lastDeniedAt: null,
      },
    ]
    adminUsers.ipTotal.value = 1

    await adminUsers.clearIPAddresses(user.id)

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      `/api/v1/admin/users/${user.id}/ips`,
      { method: 'DELETE' },
    )
    expect(adminUsers.ipAddresses.value).toEqual([])
    expect(adminUsers.ipTotal.value).toBe(0)
  })

  it('updates a selected user note and reloads users', async () => {
    const notedUser = { ...user, note: 'Follow up before renewal.' }
    apiFetch
      .mockResolvedValueOnce(userResponse([user]))
      .mockResolvedValueOnce(notedUser)
      .mockResolvedValueOnce(userResponse([notedUser]))

    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))

    const updated = await adminUsers.updateUserNote(
      user.id,
      'Follow up before renewal.',
    )

    expect(updated).toEqual(notedUser)
    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      `/api/v1/admin/users/${user.id}/note`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ note: 'Follow up before renewal.' }),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      '/api/v1/admin/users?page=1&pageSize=10&sortBy=lastLoginAt&sortDir=desc',
    )
    expect(adminUsers.users.value).toEqual([notedUser])
  })

  it('loads group options across all pages', async () => {
    apiFetch
      .mockResolvedValueOnce(userResponse([]))
      .mockResolvedValueOnce(groupResponse([group], 2))
      .mockResolvedValueOnce(groupResponse([{ ...group, id: 'group-id-2' }], 2))

    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))

    await adminUsers.loadGroups()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/groups?page=1&pageSize=100&sortBy=name&sortDir=asc',
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      '/api/v1/admin/groups?page=2&pageSize=100&sortBy=name&sortDir=asc',
    )
    expect(adminUsers.groupOptions.value.map((item) => item.id)).toEqual([
      'group-id',
      'group-id-2',
    ])
  })

  it('maps token API errors to readable messages', () => {
    expect(toAdminUserErrorMessage(apiError('token_not_found'))).toBe(
      'Virtual key no longer exists.',
    )
  })
})
