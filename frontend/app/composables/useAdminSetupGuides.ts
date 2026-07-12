import type { SetupGuide, SetupGuidePayload } from '~/types/setup-guides'

export function useAdminSetupGuides() {
  const apiFetch = useApiFetch()
  const guides = ref<SetupGuide[]>([])
  const selectedGuide = shallowRef<SetupGuide | null>(null)
  const loading = shallowRef(false)
  const saving = shallowRef(false)
  const error = shallowRef<string | null>(null)

  async function reload() {
    loading.value = true
    error.value = null
    try {
      guides.value = (
        await apiFetch<{ items: SetupGuide[] }>('/api/v1/admin/setup-guides')
      ).items
    } catch (cause) {
      error.value = toApiErrorMessage(cause, {}, 'Unable to load setup guides.')
    } finally {
      loading.value = false
    }
  }
  async function load(id: string) {
    selectedGuide.value = await apiFetch<SetupGuide>(
      `/api/v1/admin/setup-guides/${id}`,
    )
  }
  async function save(payload: SetupGuidePayload) {
    saving.value = true
    try {
      if (selectedGuide.value)
        await apiFetch(`/api/v1/admin/setup-guides/${selectedGuide.value.id}`, {
          method: 'PATCH',
          body: payload,
        })
      else
        await apiFetch('/api/v1/admin/setup-guides', {
          method: 'POST',
          body: payload,
        })
      await reload()
    } finally {
      saving.value = false
    }
  }
  async function remove(id: string) {
    await apiFetch(`/api/v1/admin/setup-guides/${id}`, { method: 'DELETE' })
    await reload()
  }
  async function reorder(ids: string[]) {
    guides.value = (
      await apiFetch<{ items: SetupGuide[] }>(
        '/api/v1/admin/setup-guides/reorder',
        { method: 'PUT', body: { ids } },
      )
    ).items
  }
  onMounted(reload)
  return {
    guides,
    selectedGuide,
    loading: readonly(loading),
    saving: readonly(saving),
    error: readonly(error),
    reload,
    load,
    save,
    remove,
    reorder,
  }
}
