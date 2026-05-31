import type { AccessGroup } from '~/types/groups'
import { toApiErrorMessage } from '~/utils/api-error'

// toProfileGroupsErrorMessage converts profile group API errors into user-facing text.
export function toProfileGroupsErrorMessage(error: unknown) {
  return toApiErrorMessage(error, {}, 'Unexpected profile groups error.')
}

// useProfileGroups loads access groups for the authenticated profile page.
export function useProfileGroups() {
  const apiFetch = useApiFetch()

  const groups = shallowRef<AccessGroup[]>([])
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)

  async function loadGroups() {
    loading.value = true
    error.value = null

    try {
      groups.value = await apiFetch<AccessGroup[]>('/api/v1/me/groups')
    } catch (fetchError) {
      groups.value = []
      error.value = toProfileGroupsErrorMessage(fetchError)
    } finally {
      loading.value = false
    }
  }

  async function reload() {
    await loadGroups()
  }

  void loadGroups()

  return {
    error,
    groups,
    loading,
    reload,
  }
}
