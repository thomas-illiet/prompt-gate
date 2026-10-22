import { describe, expect, it } from 'vitest'

import type { AdminUser } from '../../app/types/users'
import { usersToCsv } from '../../app/utils/users-csv'

const user: AdminUser = {
  activeInLast30Days: true,
  quotaResetCredits: 1,
  id: 'user-id',
  sub: 'oidc-sub',
  preferredUsername: '=unsafe',
  email: 'ada@example.com',
  name: 'Lovelace, "Ada"',
  role: 'user',
  note: 'not exported',
  isActive: true,
  lastLoginAt: '2026-01-02T00:00:00Z',
  inputTokens: 123,
  outputTokens: 456,
  expiresAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-03T00:00:00Z',
}

describe('usersToCsv', () => {
  it('exports user fields safely without internal identity or notes', () => {
    const csv = usersToCsv([user])

    expect(csv).toContain('"ID","Name","Username","Email"')
    expect(csv).toContain('"Access","Platform activity (30d)"')
    expect(csv).toContain('"Lovelace, ""Ada"""')
    expect(csv).toContain('"\'=unsafe"')
    expect(csv).toContain('"Enabled","Active"')
    expect(csv).not.toContain('oidc-sub')
    expect(csv).not.toContain('not exported')
  })

  it('exports disabled, recently inactive, and never-linked values clearly', () => {
    const csv = usersToCsv([
      {
        ...user,
        sub: null,
        email: '',
        name: '',
        isActive: false,
        activeInLast30Days: false,
        lastLoginAt: null,
      },
    ])

    expect(csv).toContain('"Disabled","Inactive"')
    expect(csv).toContain('"Never"')
    expect(csv).not.toContain('oidc-sub')
  })
})
