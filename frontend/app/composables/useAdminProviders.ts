import type {
  CreateProviderPayload,
  Provider,
  ProviderListResponse,
  UpdateProviderPayload,
} from '~/types/providers'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_name: 'Name must use lowercase letters, numbers, and single hyphens.',
  invalid_sort: 'Selected provider sort is invalid.',
  invalid_type: 'Selected provider type is invalid.',
  invalid_url: 'Base URL must be a valid HTTP or HTTPS URL.',
  name_conflict: 'Another provider already uses this name.',
  provider_not_found: 'Provider no longer exists.',
}

// toAdminProviderErrorMessage converts provider API errors into user-facing text.
export function toAdminProviderErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected provider management error.',
  )
}

// useAdminProviders coordinates provider list state and mutations.
export function useAdminProviders() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()

  const saving = shallowRef(false)
  const selectedProvider = shallowRef<Provider | null>(null)

  const queryList = useQueryList<Provider>({
    fetch: (queryString) =>
      apiFetch<ProviderListResponse>(`/api/v1/admin/providers?${queryString}`),
    initialSortBy: 'name',
    initialSortDir: 'asc',
    toErrorMessage: toAdminProviderErrorMessage,
  })

  const enabledProvidersCount = computed(
    () => queryList.items.value.filter((provider) => provider.enabled).length,
  )

  // fetchProviders refreshes providers through the shared list composable.
  async function fetchProviders() {
    await queryList.reload()
  }

  // reload exposes a stable refresh action for provider views.
  async function reload() {
    await fetchProviders()
  }

  // createProvider stores a new provider and reloads the list.
  async function createProvider(payload: CreateProviderPayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Provider created.',
        toErrorMessage: toAdminProviderErrorMessage,
      },
      async () => {
        const provider = await apiJson<Provider>(
          '/api/v1/admin/providers',
          payload,
          { method: 'POST' },
        )

        await fetchProviders()
        return provider
      },
    )
  }

  // loadProvider fetches one provider for editing.
  async function loadProvider(providerId: string) {
    selectedProvider.value = await apiFetch<Provider>(
      `/api/v1/admin/providers/${providerId}`,
    )
    return selectedProvider.value
  }

  // updateProvider patches a provider and keeps the selected copy fresh.
  async function updateProvider(
    providerId: string,
    payload: UpdateProviderPayload,
  ) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Provider updated.',
        toErrorMessage: toAdminProviderErrorMessage,
      },
      async () => {
        const provider = await apiJson<Provider>(
          `/api/v1/admin/providers/${providerId}`,
          payload,
          { method: 'PATCH' },
        )

        selectedProvider.value = provider
        await fetchProviders()
        return provider
      },
    )
  }

  // deleteProvider removes a provider and refreshes the list.
  async function deleteProvider(providerId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Provider deleted.',
        toErrorMessage: toAdminProviderErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(`/api/v1/admin/providers/${providerId}`, {
          method: 'DELETE',
        })
        await fetchProviders()
      },
    )
  }

  return {
    createProvider,
    deleteProvider,
    enabledProvidersCount,
    listError: queryList.listError,
    loadProvider,
    loading: queryList.loading,
    page: queryList.page,
    pageSize: queryList.pageSize,
    providers: queryList.items,
    reload,
    saving,
    selectedProvider,
    setPage: queryList.setPage,
    setPageSize: queryList.setPageSize,
    setSort: queryList.setSort,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    total: queryList.total,
    updateProvider,
  }
}
