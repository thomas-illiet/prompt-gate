import { FetchError } from 'ofetch'

import { BLOCKED_ROUTE_PATH, resolveRuntimeApiBaseUrl } from '~/utils/auth'

type ApiFetchOptions = NonNullable<Parameters<typeof $fetch>[1]> & {
  headers?: HeadersInit
}

// useApiFetch returns an authenticated API fetcher with access redirects.
export function useApiFetch() {
  const runtimeConfig = useRuntimeConfig()
  const route = useRoute()
  const authStore = useAuthStore()

  // apiFetch waits for auth readiness, performs the request, and handles 401/403.
  return async function apiFetch<T>(
    request: string,
    options: ApiFetchOptions = {},
  ) {
    await authStore.waitUntilReady()

    if (!authStore.isAuthenticated) {
      await navigateTo({
        path: '/login',
        query: { redirect: route.fullPath },
      })

      throw createError({
        statusCode: 401,
        statusMessage: 'Authentication required.',
      })
    }

    try {
      return await $fetch<T>(request, {
        ...options,
        baseURL: resolveRuntimeApiBaseUrl(runtimeConfig.public.apiBaseUrl),
        credentials: 'include',
      } as Parameters<typeof $fetch>[1])
    } catch (error) {
      if (error instanceof FetchError && error.response?.status === 401) {
        authStore.clearSession()

        await navigateTo({
          path: '/login',
          query: { redirect: route.fullPath },
        })
      }

      if (error instanceof FetchError && error.response?.status === 403) {
        const stillAuthenticated = await authStore.refresh()

        if (!stillAuthenticated) {
          await navigateTo({
            path: '/login',
            query: { redirect: route.fullPath },
          })
        } else {
          await navigateTo(BLOCKED_ROUTE_PATH)
        }
      }

      throw error
    }
  }
}
