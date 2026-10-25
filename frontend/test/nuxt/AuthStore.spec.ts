import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '../../app/stores/auth'

const { fetchMock, useRuntimeConfigMock } = vi.hoisted(() => ({
  fetchMock: vi.fn(),
  useRuntimeConfigMock: vi.fn(() => ({
    app: {
      baseURL: '/',
      buildAssetsDir: '/_nuxt/',
      cdnURL: '',
    },
    public: { apiBaseUrl: 'https://api.example.com/' },
  })),
}))

mockNuxtImport('$fetch', () => fetchMock)
mockNuxtImport('useRuntimeConfig', () => useRuntimeConfigMock)

const sessionUser = {
  email: 'limited@example.com',
  id: 'limited-id',
  isActive: true,
  lastLoginAt: '2026-10-25T08:00:00Z',
  name: 'Limited User',
  preferredUsername: 'limited-user',
  role: 'limited',
  sub: 'limited-subject',
} as const

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    fetchMock.mockReset()
    useRuntimeConfigMock.mockClear()
  })

  it('keeps the administrator contact separate from the authenticated user', async () => {
    fetchMock.mockResolvedValue({
      ...sessionUser,
      adminContactEmail: ' admin@example.com ',
    })
    const store = useAuthStore()

    await expect(store.refresh()).resolves.toBe(true)

    expect(store.adminContactEmail).toBe('admin@example.com')
    expect(store.user).toEqual(sessionUser)
    expect(store.user).not.toHaveProperty('adminContactEmail')
    expect(fetchMock).toHaveBeenCalledWith('/auth/session', {
      baseURL: 'https://api.example.com',
      credentials: 'include',
    })

    store.clearSession()
    expect(store.adminContactEmail).toBeNull()
  })

  it('supports session responses from deployments without a configured contact', async () => {
    fetchMock.mockResolvedValue(sessionUser)
    const store = useAuthStore()

    await expect(store.refresh()).resolves.toBe(true)

    expect(store.adminContactEmail).toBeNull()
    expect(store.user).toEqual(sessionUser)
  })
})
