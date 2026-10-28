import type { InstanceResponse } from '~/types/instance'
import { isInstanceEnvironment } from '~/types/instance'
import { resolveRuntimeApiBaseUrl } from '~/utils/auth'

export const useInstanceStore = defineStore('instance', () => {
  const environment = shallowRef<InstanceResponse['environment'] | 'unknown'>(
    'unknown',
  )
  const isReady = shallowRef(false)
  let initializationPromise: Promise<void> | null = null

  async function initialize() {
    if (initializationPromise) return initializationPromise

    initializationPromise = (async () => {
      if (!import.meta.client) {
        isReady.value = true
        return
      }

      const baseURL = resolveRuntimeApiBaseUrl(
        useRuntimeConfig().public.apiBaseUrl,
      )
      if (!baseURL) {
        isReady.value = true
        return
      }

      try {
        const response = await $fetch<InstanceResponse>(
          '/api/v1/public/instance',
          { baseURL },
        )
        if (isInstanceEnvironment(response.environment)) {
          environment.value = response.environment
        }
      } finally {
        isReady.value = true
      }
    })().catch(() => {
      isReady.value = true
    })

    return initializationPromise
  }

  return { environment, initialize, isReady }
})
