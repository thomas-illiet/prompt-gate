import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import {
  toAdminUserErrorMessage,
  useAdminUsers,
} from '../../app/composables/useAdminUsers'
import type {
  FirewallRule,
  FirewallRuleListResponse,
  FirewallRulePayload,
} from '../../app/types/firewall'
import type {
  UserToken,
  UserTokenListResponse,
} from '../../app/types/user-service'
import type { AccessGroup, GroupListResponse } from '../../app/types/groups'
import type { AdminUser, UserListResponse } from '../../app/types/users'

const { apiFetch, useApiFetchMock, useRouteMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return {
    apiFetch,
    useApiFetchMock: vi.fn(() => apiFetch),
    useRouteMock: vi.fn(() => ({ fullPath: '/admin/users' })),
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
  firewallOverrideEnabled: false,
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

const firewallRule: FirewallRule = {
  id: 'firewall-rule-id',
  userId: user.id,
  address: '10.0.0.10',
  description: 'Home office',
  priority: 1,
  action: 'allow',
  enabled: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const firewallPayload: FirewallRulePayload = {
  address: '10.0.0.10',
  description: 'Home office',
  priority: 1,
  action: 'allow',
  enabled: true,
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

function firewallResponse(
  items: FirewallRule[],
  total = items.length,
): FirewallRuleListResponse {
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
    setActivePinia(createPinia())
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

  it('manages scoped user firewall rules', async () => {
    apiFetch
      .mockResolvedValueOnce(userResponse([]))
      .mockResolvedValueOnce(firewallResponse([firewallRule]))
      .mockResolvedValueOnce(firewallRule)
      .mockResolvedValueOnce(firewallResponse([firewallRule]))
      .mockResolvedValueOnce(firewallRule)
      .mockResolvedValueOnce(firewallResponse([firewallRule]))
      .mockResolvedValueOnce(firewallRule)
      .mockResolvedValueOnce(firewallResponse([firewallRule]))
      .mockResolvedValueOnce({ allowed: true, matchedRule: firewallRule })
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(firewallResponse([]))

    const adminUsers = useAdminUsers()
    await vi.waitFor(() => expect(adminUsers.loading.value).toBe(false))

    await adminUsers.loadFirewallRules(user.id)
    await adminUsers.createFirewallRule(user.id, firewallPayload)
    await adminUsers.updateFirewallRule(
      user.id,
      firewallRule.id,
      firewallPayload,
    )
    await adminUsers.moveFirewallRulePriority(
      user.id,
      firewallRule.id,
      'increase',
    )
    const simulation = await adminUsers.simulateFirewallIp(user.id, '10.0.0.10')
    await adminUsers.deleteFirewallRule(user.id, firewallRule.id)

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      `/api/v1/admin/users/${user.id}/firewall/rules?page=1&pageSize=10&sortBy=priority&sortDir=asc`,
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      `/api/v1/admin/users/${user.id}/firewall/rules`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(firewallPayload),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      5,
      `/api/v1/admin/users/${user.id}/firewall/rules/${firewallRule.id}`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(firewallPayload),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      7,
      `/api/v1/admin/users/${user.id}/firewall/rules/${firewallRule.id}/priority`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ direction: 'increase' }),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      9,
      `/api/v1/admin/users/${user.id}/firewall/simulate`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ clientIp: '10.0.0.10' }),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      10,
      `/api/v1/admin/users/${user.id}/firewall/rules/${firewallRule.id}`,
      { method: 'DELETE' },
    )
    expect(simulation.allowed).toBe(true)
    expect(adminUsers.firewallRules.value).toEqual([])
  })

  it('maps token API errors to readable messages', () => {
    expect(toAdminUserErrorMessage(apiError('token_not_found'))).toBe(
      'Virtual key no longer exists.',
    )
  })
})
