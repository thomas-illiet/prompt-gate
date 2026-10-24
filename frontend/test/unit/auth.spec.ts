import { describe, expect, it } from 'vitest'

import {
  APP_ROLES,
  appRoleLabel,
  hasRequiredRole,
  isAuthUser,
  isBlockedUser,
  publicLoginErrorMessage,
  resolveRuntimeApiBaseUrl,
} from '../../app/utils/auth'

describe('resolveRuntimeApiBaseUrl', () => {
  it('uses the configured API base URL when provided', () => {
    expect(
      resolveRuntimeApiBaseUrl(
        ' https://api.example.com/ ',
        'http://localhost:8080',
      ),
    ).toBe('https://api.example.com')
  })

  it('falls back to the current frontend origin for same-origin deployments', () => {
    expect(resolveRuntimeApiBaseUrl('', 'http://localhost:8080/')).toBe(
      'http://localhost:8080',
    )
  })

  it('returns an empty URL when neither source is available', () => {
    expect(resolveRuntimeApiBaseUrl('', null)).toBe('')
  })
})

describe('isAuthUser', () => {
  const session = {
    email: 'ada@example.com',
    id: 'user-1',
    isActive: true,
    lastLoginAt: '2026-01-01T00:00:00Z',
    name: 'Ada Lovelace',
    preferredUsername: 'ada',
    role: 'user',
    sub: 'oidc-ada',
  }

  it('accepts a complete authenticated session', () => {
    expect(isAuthUser(session)).toBe(true)
    expect(isAuthUser({ ...session, role: 'limited' })).toBe(true)
  })

  it('rejects HTML and incomplete session payloads', () => {
    expect(isAuthUser('<!doctype html>')).toBe(false)
    expect(isAuthUser({ ...session, isActive: undefined })).toBe(false)
    expect(isAuthUser({ ...session, role: 'owner' })).toBe(false)
    expect(isAuthUser({ ...session, role: 'manager' })).toBe(false)
  })
})

describe('application roles', () => {
  it('exposes Limited and removes Manager from selectable roles', () => {
    expect(APP_ROLES).toEqual(['none', 'limited', 'user', 'admin'])
    expect(appRoleLabel('limited')).toBe('Limited')
  })
})

describe('access-state classification', () => {
  const activeUser = {
    email: 'ada@example.com',
    id: 'user-1',
    isActive: true,
    lastLoginAt: '2026-01-01T00:00:00Z',
    name: 'Ada Lovelace',
    preferredUsername: 'ada',
    role: 'user' as const,
    sub: 'oidc-ada',
  }

  it('keeps an absent session separate from a blocked account', () => {
    expect(isBlockedUser(null)).toBe(false)
  })

  it('blocks inactive accounts and accounts without an application role', () => {
    expect(isBlockedUser({ ...activeUser, isActive: false })).toBe(true)
    expect(isBlockedUser({ ...activeUser, role: 'none' })).toBe(true)
  })

  it('distinguishes a valid session from insufficient route permissions', () => {
    expect(hasRequiredRole(activeUser, ['limited', 'user'])).toBe(true)
    expect(hasRequiredRole(activeUser, ['admin'])).toBe(false)
  })
})

describe('publicLoginErrorMessage', () => {
  it('maps callback values to trusted product copy', () => {
    expect(publicLoginErrorMessage('access_denied')).toContain('cancelled')
    expect(publicLoginErrorMessage('raw provider detail')).toBe(
      'PromptGate could not complete sign-in. Please try again.',
    )
  })

  it('hides raw initialization failures behind a stable message', () => {
    expect(publicLoginErrorMessage(null, true)).toContain(
      'cannot reach the authentication service',
    )
    expect(publicLoginErrorMessage(null)).toBeNull()
  })
})
