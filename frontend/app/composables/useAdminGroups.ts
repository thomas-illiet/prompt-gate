import type {
  AccessGroup,
  CreateGroupPayload,
  GroupListResponse,
  GroupMemberSummary,
  GroupModelPatternValidationPayload,
  GroupModelPatternValidationResponse,
  UpdateGroupPayload,
} from '~/types/groups'
import type { Provider } from '~/types/providers'
import type { ServiceAccount } from '~/types/service-accounts'
import type { AdminUser } from '~/types/users'
import { Notify } from '~/stores/notification'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  group_not_found: 'Group no longer exists.',
  invalid_name: 'Name must use lowercase letters, numbers, and single hyphens.',
  invalid_display_name: 'Display name is required.',
  invalid_regex: 'One or more model patterns are invalid regular expressions.',
  invalid_sort: 'Selected group sort is invalid.',
  name_conflict: 'Another group already uses this name.',
  provider_required: 'Select at least one provider.',
  provider_not_found: 'Provider no longer exists.',
  user_not_found: 'User or service account no longer exists.',
}

export function toAdminGroupErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected group management error.',
  )
}

export function useAdminGroups() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()

  const saving = shallowRef(false)
  const memberLoading = shallowRef(false)
  const modelValidationLoading = shallowRef(false)
  const modelValidation =
    shallowRef<GroupModelPatternValidationResponse | null>(null)
  const modelValidationError = shallowRef<string | null>(null)
  const selectedGroup = shallowRef<AccessGroup | null>(null)
  const providerOptions = shallowRef<Provider[]>([])
  const memberOptions = shallowRef<GroupMemberSummary[]>([])

  async function loadAllPages<T>(path: string, sortBy = 'name') {
    const items: T[] = []
    const pageSize = 100

    for (let page = 1; ; page += 1) {
      const params = new URLSearchParams({
        page: page.toString(),
        pageSize: pageSize.toString(),
        sortBy,
        sortDir: 'asc',
      })
      const response = await apiFetch<{
        items: T[]
        total: number
      }>(`${path}?${params.toString()}`)
      items.push(...response.items)
      if (items.length >= response.total || response.items.length === 0) {
        return items
      }
    }
  }

  const queryList = useQueryList<AccessGroup>({
    fetch: (queryString) =>
      apiFetch<GroupListResponse>(`/api/v1/admin/groups?${queryString}`),
    initialSortBy: 'name',
    initialSortDir: 'asc',
    toErrorMessage: toAdminGroupErrorMessage,
  })

  async function reload() {
    await queryList.reload()
  }

  async function loadProviderOptions() {
    providerOptions.value = await loadAllPages<Provider>(
      '/api/v1/admin/providers',
    )
    return providerOptions.value
  }

  async function loadMemberOptions() {
    memberLoading.value = true
    try {
      const [users, serviceAccounts] = await Promise.all([
        loadAllPages<AdminUser>('/api/v1/admin/users'),
        loadAllPages<ServiceAccount>('/api/v1/admin/service-accounts'),
      ])

      memberOptions.value = [
        ...users.map((user) => ({
          id: user.id,
          preferredUsername: user.preferredUsername,
          email: user.email,
          name: user.name,
          type: 'user' as const,
          role: user.role,
          isActive: user.isActive,
        })),
        ...serviceAccounts.map((account) => ({
          id: account.id,
          preferredUsername: account.identifier,
          email: '',
          name: account.name,
          type: 'service' as const,
          role: account.role,
          isActive: account.isActive,
        })),
      ]
      return memberOptions.value
    } catch (error) {
      Notify.error(toAdminGroupErrorMessage(error))
      throw error
    } finally {
      memberLoading.value = false
    }
  }

  async function loadGroup(groupId: string) {
    selectedGroup.value = await apiFetch<AccessGroup>(
      `/api/v1/admin/groups/${groupId}`,
    )
    return selectedGroup.value
  }

  async function createGroup(payload: CreateGroupPayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Group created.',
        toErrorMessage: toAdminGroupErrorMessage,
      },
      async () => {
        const group = await apiJson<AccessGroup>(
          '/api/v1/admin/groups',
          payload,
          { method: 'POST' },
        )
        await reload()
        return group
      },
    )
  }

  async function updateGroup(groupId: string, payload: UpdateGroupPayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Group updated.',
        toErrorMessage: toAdminGroupErrorMessage,
      },
      async () => {
        const group = await apiJson<AccessGroup>(
          `/api/v1/admin/groups/${groupId}`,
          payload,
          { method: 'PATCH' },
        )
        selectedGroup.value = group
        await reload()
        return group
      },
    )
  }

  async function deleteGroup(groupId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Group deleted.',
        toErrorMessage: toAdminGroupErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(`/api/v1/admin/groups/${groupId}`, {
          method: 'DELETE',
        })
        await reload()
      },
    )
  }

  async function addMember(groupId: string, userId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Member added.',
        toErrorMessage: toAdminGroupErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(
          `/api/v1/admin/groups/${groupId}/members/${userId}`,
          { method: 'PUT' },
        )
        await loadGroup(groupId)
        await reload()
      },
    )
  }

  async function removeMember(groupId: string, userId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Member removed.',
        toErrorMessage: toAdminGroupErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(
          `/api/v1/admin/groups/${groupId}/members/${userId}`,
          { method: 'DELETE' },
        )
        await loadGroup(groupId)
        await reload()
      },
    )
  }

  function clearModelValidation() {
    modelValidation.value = null
    modelValidationError.value = null
  }

  async function validateModelPatterns(
    payload: GroupModelPatternValidationPayload,
  ) {
    modelValidationLoading.value = true
    modelValidationError.value = null
    try {
      modelValidation.value =
        await apiJson<GroupModelPatternValidationResponse>(
          '/api/v1/admin/groups/model-patterns/validate',
          payload,
          { method: 'POST' },
        )
      return modelValidation.value
    } catch (error) {
      modelValidation.value = null
      modelValidationError.value = toAdminGroupErrorMessage(error)
      throw error
    } finally {
      modelValidationLoading.value = false
    }
  }

  return {
    addMember,
    clearModelValidation,
    createGroup,
    deleteGroup,
    groups: queryList.items,
    listError: queryList.listError,
    loadGroup,
    loadMemberOptions,
    loadProviderOptions,
    loading: queryList.loading,
    memberLoading,
    memberOptions,
    modelValidation,
    modelValidationError,
    modelValidationLoading,
    page: queryList.page,
    pageSize: queryList.pageSize,
    providerOptions,
    reload,
    removeMember,
    saving,
    search: queryList.search,
    selectedGroup,
    setPage: queryList.setPage,
    setPageSize: queryList.setPageSize,
    setSearch: queryList.setSearch,
    setSort: queryList.setSort,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    total: queryList.total,
    updateGroup,
    validateModelPatterns,
  }
}
