import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useApiFetch } from '../../app/composables/useApiFetch'

const {
  authStore,
  clearSessionMock,
  fetchMock,
  navigateToMock,
  refreshMock,
  useAuthStoreMock,
  useRouteMock,
  useRuntimeConfigMock,
  waitUntilReadyMock,
} = vi.hoisted(() => {
  const waitUntilReadyMock = vi.fn(async () => undefined)
  const clearSessionMock = vi.fn()
  const refreshMock = vi.fn(async () => true)
  const authStore = {
    clearSession: clearSessionMock,
    isAuthenticated: true,
    refresh: refreshMock,
    waitUntilReady: waitUntilReadyMock,
  }

  return {
    authStore,
    clearSessionMock,
    fetchMock: vi.fn(),
    navigateToMock: vi.fn(async () => undefined),
    refreshMock,
    useAuthStoreMock: vi.fn(() => authStore),
    useRouteMock: vi.fn(() => ({ fullPath: '/admin/users' })),
    useRuntimeConfigMock: vi.fn(() => ({
      app: {
        baseURL: '/',
        buildAssetsDir: '/_nuxt/',
        cdnURL: '',
      },
      public: { apiBaseUrl: 'https://api.example.com/' },
    })),
    waitUntilReadyMock,
  }
})

mockNuxtImport('navigateTo', () => navigateToMock)
mockNuxtImport('useAuthStore', () => useAuthStoreMock)
mockNuxtImport('useRoute', () => useRouteMock)
mockNuxtImport('useRuntimeConfig', () => useRuntimeConfigMock)

function fetchError(status: number) {
  return Object.assign(Object.create(FetchError.prototype), {
    response: { status },
  }) as FetchError
}

describe('useApiFetch', () => {
  beforeEach(() => {
    authStore.isAuthenticated = true
    clearSessionMock.mockClear()
    fetchMock.mockReset()
    navigateToMock.mockClear()
    refreshMock.mockReset()
    refreshMock.mockResolvedValue(true)
    useAuthStoreMock.mockClear()
    useRouteMock.mockClear()
    useRuntimeConfigMock.mockClear()
    waitUntilReadyMock.mockClear()
    vi.stubGlobal('$fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('clears the session and redirects to login on 401', async () => {
    const error = fetchError(401)
    fetchMock.mockRejectedValueOnce(error)

    const apiFetch = useApiFetch()

    await expect(apiFetch('/api/v1/me')).rejects.toBe(error)

    expect(waitUntilReadyMock).toHaveBeenCalledOnce()
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/me', {
      baseURL: 'https://api.example.com',
      credentials: 'include',
    })
    expect(clearSessionMock).toHaveBeenCalledOnce()
    expect(navigateToMock).toHaveBeenCalledWith({
      path: '/login',
      query: { redirect: '/admin/users' },
    })
  })

  it('refreshes auth and redirects blocked authenticated users on 403', async () => {
    const error = fetchError(403)
    fetchMock.mockRejectedValueOnce(error)
    refreshMock.mockResolvedValueOnce(true)

    const apiFetch = useApiFetch()

    await expect(apiFetch('/api/v1/admin/users')).rejects.toBe(error)

    expect(refreshMock).toHaveBeenCalledOnce()
    expect(navigateToMock).toHaveBeenCalledWith('/access-denied')
  })

  it('redirects to login on 403 when refresh loses the session', async () => {
    const error = fetchError(403)
    fetchMock.mockRejectedValueOnce(error)
    refreshMock.mockResolvedValueOnce(false)

    const apiFetch = useApiFetch()

    await expect(apiFetch('/api/v1/admin/users')).rejects.toBe(error)

    expect(refreshMock).toHaveBeenCalledOnce()
    expect(navigateToMock).toHaveBeenCalledWith({
      path: '/login',
      query: { redirect: '/admin/users' },
    })
  })
})
