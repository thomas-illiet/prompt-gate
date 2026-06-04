import { describe, expect, it } from 'vitest'

import {
  adminGroupPath,
  adminGroupsPath,
  adminServiceAccountPath,
  adminServiceAccountsPath,
  adminUserPath,
  adminUsersPath,
  withApiQuery,
} from '../../app/utils/api-paths'

describe('api path helpers', () => {
  it('builds stable collection paths', () => {
    expect(adminGroupsPath).toBe('/api/v1/admin/groups')
    expect(adminServiceAccountsPath).toBe('/api/v1/admin/service-accounts')
    expect(adminUsersPath).toBe('/api/v1/admin/users')
  })

  it('encodes dynamic path segments', () => {
    expect(adminUserPath('user/with space', 'tokens', 'token/id')).toBe(
      '/api/v1/admin/users/user%2Fwith%20space/tokens/token%2Fid',
    )
    expect(
      adminServiceAccountPath('service account', 'firewall', 'rules', '1/2'),
    ).toBe(
      '/api/v1/admin/service-accounts/service%20account/firewall/rules/1%2F2',
    )
    expect(adminGroupPath('group/id')).toBe('/api/v1/admin/groups/group%2Fid')
  })

  it('appends query strings only when present', () => {
    const params = new URLSearchParams({ page: '1', sortBy: 'name' })

    expect(withApiQuery(adminUsersPath, params)).toBe(
      '/api/v1/admin/users?page=1&sortBy=name',
    )
    expect(withApiQuery(adminUsersPath, new URLSearchParams())).toBe(
      '/api/v1/admin/users',
    )
  })
})
