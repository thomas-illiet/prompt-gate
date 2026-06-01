import type {
  MonitoringService,
  MonitoringServiceListResponse,
  MonitoringServicePayload,
} from '~/types/monitoring'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_interval: 'Check interval must be between 30 seconds and 24 hours.',
  invalid_name: 'Name must use lowercase letters, numbers, and single hyphens.',
  invalid_sort: 'Selected monitoring sort is invalid.',
  invalid_status_code: 'Expected HTTP status code must be between 100 and 599.',
  invalid_url: 'Service URL must be a valid HTTP or HTTPS URL.',
  monitoring_service_not_found: 'Monitoring service no longer exists.',
  name_conflict: 'Another monitoring service already uses this name.',
}

// toAdminMonitoringErrorMessage converts monitoring API errors into readable text.
export function toAdminMonitoringErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected monitoring management error.',
  )
}

// useAdminMonitoring coordinates monitoring service list state and mutations.
export function useAdminMonitoring() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()

  const saving = shallowRef(false)
  const selectedService = shallowRef<MonitoringService | null>(null)

  const queryList = useQueryList<MonitoringService>({
    fetch: (queryString) =>
      apiFetch<MonitoringServiceListResponse>(
        `/api/v1/admin/monitoring/services?${queryString}`,
      ),
    initialSortBy: 'name',
    initialSortDir: 'asc',
    toErrorMessage: toAdminMonitoringErrorMessage,
  })

  const enabledServicesCount = computed(
    () => queryList.items.value.filter((service) => service.enabled).length,
  )
  const degradedServicesCount = computed(
    () =>
      queryList.items.value.filter(
        (service) => service.enabled && service.status === 'degraded',
      ).length,
  )

  async function fetchServices() {
    await queryList.reload()
  }

  async function reload() {
    await fetchServices()
  }

  async function createService(payload: MonitoringServicePayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Monitoring service created.',
        toErrorMessage: toAdminMonitoringErrorMessage,
      },
      async () => {
        const service = await apiJson<MonitoringService>(
          '/api/v1/admin/monitoring/services',
          payload,
          { method: 'POST' },
        )

        await fetchServices()
        return service
      },
    )
  }

  async function loadService(serviceId: string) {
    selectedService.value = await apiFetch<MonitoringService>(
      `/api/v1/admin/monitoring/services/${serviceId}`,
    )
    return selectedService.value
  }

  async function updateService(
    serviceId: string,
    payload: MonitoringServicePayload,
  ) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Monitoring service updated.',
        toErrorMessage: toAdminMonitoringErrorMessage,
      },
      async () => {
        const service = await apiJson<MonitoringService>(
          `/api/v1/admin/monitoring/services/${serviceId}`,
          payload,
          { method: 'PATCH' },
        )

        selectedService.value = service
        await fetchServices()
        return service
      },
    )
  }

  async function deleteService(serviceId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Monitoring service deleted.',
        toErrorMessage: toAdminMonitoringErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(
          `/api/v1/admin/monitoring/services/${serviceId}`,
          { method: 'DELETE' },
        )
        await fetchServices()
      },
    )
  }

  async function checkService(serviceId: string) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Monitoring check completed.',
        toErrorMessage: toAdminMonitoringErrorMessage,
      },
      async () => {
        const service = await apiFetch<MonitoringService>(
          `/api/v1/admin/monitoring/services/${serviceId}/check`,
          { method: 'POST' },
        )

        if (selectedService.value?.id === serviceId) {
          selectedService.value = service
        }
        await fetchServices()
        return service
      },
    )
  }

  return {
    checkService,
    createService,
    degradedServicesCount,
    deleteService,
    enabledServicesCount,
    listError: queryList.listError,
    loadService,
    loading: queryList.loading,
    page: queryList.page,
    pageSize: queryList.pageSize,
    reload,
    saving,
    selectedService,
    services: queryList.items,
    setPage: queryList.setPage,
    setPageSize: queryList.setPageSize,
    setSort: queryList.setSort,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    total: queryList.total,
    updateService,
  }
}
