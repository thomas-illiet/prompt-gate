import { FetchError } from 'ofetch'

import type { AuthUser } from '~/types/auth'
import {
  DEFAULT_AUTH_REDIRECT_PATH,
  FRONTEND_ORIGIN_QUERY_PARAM,
  currentFrontendOrigin,
  normalizeRedirectPath,
  resolveRuntimeApiBaseUrl,
  sanitizeRedirectPath,
} from '~/utils/auth'

const DEFAULT_LOGOUT_REDIRECT_PATH = '/login'
const MISSING_CONFIGURATION_MESSAGE = 'API base URL is missing.'

// toErrorMessage turns auth failures into displayable messages.
function toErrorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message
  }

  if (
    typeof error === 'object' &&
    error !== null &&
    ('error' in error || 'error_description' in error || 'message' in error)
  ) {
    const typedError = error as {
      error?: string
      error_description?: string
      message?: string
    }

    return (
      typedError.error_description ||
      typedError.message ||
      typedError.error ||
      'Authentication failed.'
    )
  }

  if (typeof error === 'string') {
    return error
  }

  return 'Unexpected authentication error.'
}

type SessionResponse = AuthUser

// apiBaseUrl returns the normalized backend API base URL.
function apiBaseUrl() {
  return resolveRuntimeApiBaseUrl(useRuntimeConfig().public.apiBaseUrl)
}

export const useAuthStore = defineStore('auth', () => {
  const isReady = shallowRef(false)
  const isConfigured = shallowRef(true)
  const isAuthenticated = shallowRef(false)
  const user = shallowRef<AuthUser | null>(null)
  const initializationError = shallowRef<string | null>(null)

  let initializationPromise: Promise<void> | null = null

  // resetSessionState clears local auth state and stores an optional error.
  function resetSessionState(errorMessage: string | null = null) {
    isAuthenticated.value = false
    user.value = null
    initializationError.value = errorMessage
  }

  // loadSession fetches the current backend session.
  async function loadSession() {
    const baseURL = apiBaseUrl()
    if (!baseURL) {
      markConfigurationMissing(
        'Set NUXT_PUBLIC_API_BASE_URL to enable backend authentication.',
      )
      return
    }

    try {
      const session = await $fetch<SessionResponse>('/auth/session', {
        baseURL,
        credentials: 'include',
      })

      user.value = session
      isAuthenticated.value = true
      initializationError.value = null
    } catch (error) {
      if (error instanceof FetchError && error.response?.status === 401) {
        resetSessionState()
        return
      }

      resetSessionState(toErrorMessage(error))
    }
  }

  // initialize loads the session once on the client.
  async function initialize() {
    if (initializationPromise) {
      return initializationPromise
    }

    initializationPromise = (async () => {
      if (!import.meta.client) {
        isReady.value = true
        return
      }

      await loadSession()
      isReady.value = true
    })()

    return initializationPromise
  }

  // waitUntilReady waits for the initial session load to finish.
  async function waitUntilReady() {
    if (!initializationPromise) {
      await initialize()
      return
    }

    try {
      await initializationPromise
    } catch {
      // The store already exposes the initialization error state.
    }
  }

  // markConfigurationMissing exposes missing API configuration as auth state.
  function markConfigurationMissing(message = MISSING_CONFIGURATION_MESSAGE) {
    isConfigured.value = false
    resetSessionState(message)
    isReady.value = true
  }

  // refresh reloads the session and returns whether it is still authenticated.
  async function refresh() {
    await loadSession()
    return isAuthenticated.value
  }

  // login redirects the browser to the backend login endpoint.
  async function login(redirectPath?: string) {
    const baseURL = apiBaseUrl()
    if (!baseURL) {
      throw new Error(MISSING_CONFIGURATION_MESSAGE)
    }

    const targetPath = sanitizeRedirectPath(
      redirectPath,
      DEFAULT_AUTH_REDIRECT_PATH,
    )
    const loginUrl = new URL('/auth/login', baseURL)
    loginUrl.searchParams.set('redirect', targetPath)
    const frontendOrigin = currentFrontendOrigin()
    if (frontendOrigin) {
      loginUrl.searchParams.set(FRONTEND_ORIGIN_QUERY_PARAM, frontendOrigin)
    }
    window.location.assign(loginUrl.toString())
  }

  // logout clears local state and redirects through the backend logout endpoint.
  async function logout(redirectPath = DEFAULT_LOGOUT_REDIRECT_PATH) {
    const baseURL = apiBaseUrl()
    clearSession()

    if (!baseURL) {
      return
    }

    const logoutUrl = new URL('/auth/logout', baseURL)
    logoutUrl.searchParams.set(
      'redirect',
      normalizeRedirectPath(redirectPath, DEFAULT_LOGOUT_REDIRECT_PATH),
    )
    const frontendOrigin = currentFrontendOrigin()
    if (frontendOrigin) {
      logoutUrl.searchParams.set(FRONTEND_ORIGIN_QUERY_PARAM, frontendOrigin)
    }
    window.location.assign(logoutUrl.toString())
  }

  // clearSession removes the local authenticated user.
  function clearSession() {
    resetSessionState()
  }

  return {
    clearSession,
    initialize,
    initializationError,
    isAuthenticated,
    isConfigured,
    isReady,
    login,
    logout,
    markConfigurationMissing,
    refresh,
    user,
    waitUntilReady,
  }
})
