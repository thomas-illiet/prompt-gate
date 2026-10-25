import type { Ref } from 'vue'
import { onScopeDispose, shallowRef, watch } from 'vue'

import type { DashboardQuotaRiskResponse } from '~/types/user-service'
import { toApiErrorMessage } from '~/utils/api-error'

// useAdminDashboardQuotaRisk owns the live risk summary used by the overview.
export function useAdminDashboardQuotaRisk(refreshVersion: Ref<number>) {
  const apiFetch = useApiFetch()
  const data = shallowRef<DashboardQuotaRiskResponse | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  let controller: AbortController | null = null
  let requestVersion = 0

  async function load() {
    const version = ++requestVersion
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    error.value = null

    try {
      const response = await apiFetch<DashboardQuotaRiskResponse>(
        '/api/v1/admin/dashboard/quota-risk',
        { signal: controller.signal },
      )
      if (version === requestVersion) data.value = response
    } catch (cause) {
      if (
        version === requestVersion &&
        !(cause instanceof Error && cause.name === 'AbortError')
      ) {
        error.value = toApiErrorMessage(
          cause,
          {},
          'Unable to load live quota risk.',
        )
      }
    } finally {
      if (version === requestVersion) loading.value = false
    }
  }

  watch(refreshVersion, () => void load(), { immediate: true })

  onScopeDispose(() => {
    requestVersion += 1
    controller?.abort()
  }, true)

  return { data, error, loading, reload: load }
}
