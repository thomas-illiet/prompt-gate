import type { MonitoringStatusResponse } from '~/types/monitoring'
import { toApiErrorMessage } from '~/utils/api-error'

interface MonitoringStatusOptions {
  pollMs?: number
}

// useMonitoringStatus loads the current user-visible monitoring incident state.
export function useMonitoringStatus(options: MonitoringStatusOptions = {}) {
  const apiFetch = useApiFetch()
  const pollMs = options.pollMs ?? 60000
  const status = shallowRef<MonitoringStatusResponse['status']>('ok')
  const services = shallowRef<MonitoringStatusResponse['services']>([])
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  let pollTimer: ReturnType<typeof setInterval> | null = null

  async function load() {
    loading.value = true
    error.value = null
    try {
      const response = await apiFetch<MonitoringStatusResponse>(
        '/api/v1/monitoring/status',
      )
      status.value = response.status
      services.value = response.services
    } catch (err) {
      error.value = toApiErrorMessage(err, {}, 'Monitoring status unavailable.')
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void load()
    pollTimer = setInterval(() => {
      void load()
    }, pollMs)
  })

  onUnmounted(() => {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  })

  return {
    error,
    loading,
    refresh: load,
    services,
    status,
  }
}
