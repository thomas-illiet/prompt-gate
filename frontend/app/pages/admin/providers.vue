<script setup lang="ts">
import AdminProviderDialog from '~/components/AdminProviders/AdminProviderDialog.vue'
import type {
  CreateProviderPayload,
  Provider,
  UpdateProviderPayload,
} from '~/types/providers'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Providers',
  icon: 'mdi-cloud-cog-outline',
  drawerIndex: 7,
  drawerSection: 'Infrastructure',
})

const adminProviders = useAdminProviders()
const providerDialogOpen = shallowRef(false)
const deleteDialog = useTargetDialog<Provider>()
const toggleDialog = useTargetDialog<Provider>()
const toggleConfirm = useToggleConfirmDialog(toggleDialog.target, {
  disableIcon: 'mdi-cloud-off-outline',
  enableIcon: 'mdi-cloud-check-outline',
  entityLabel: 'provider',
  fallbackMessage: 'Change this provider status.',
  isActive: (provider) => provider.enabled,
  name: (provider) => provider.name,
})

const totalLabel = computed(() => {
  const total = adminProviders.providers.value.length
  const enabled = adminProviders.enabledProvidersCount.value
  return total === 1 ? `${enabled}/1 enabled` : `${enabled}/${total} enabled`
})

// openCreateDialog prepares a blank provider form.
function openCreateDialog() {
  adminProviders.selectedProvider.value = null
  providerDialogOpen.value = true
}

// openEditDialog loads a provider before showing the edit dialog.
async function openEditDialog(provider: Provider) {
  await adminProviders.loadProvider(provider.id)
  providerDialogOpen.value = true
}

// saveProvider creates or updates the active provider form.
async function saveProvider(
  payload: CreateProviderPayload | UpdateProviderPayload,
) {
  if (adminProviders.selectedProvider.value) {
    await adminProviders.updateProvider(
      adminProviders.selectedProvider.value.id,
      payload,
    )
  } else if ('name' in payload) {
    await adminProviders.createProvider(payload)
  } else {
    return
  }

  providerDialogOpen.value = false
}

// confirmToggleProvider toggles the selected provider enabled state.
async function confirmToggleProvider() {
  if (!toggleDialog.target.value) {
    return
  }

  const provider = toggleDialog.target.value
  await adminProviders.updateProvider(provider.id, {
    displayName: provider.displayName,
    type: provider.type,
    baseUrl: provider.baseUrl,
    enabled: !provider.enabled,
  })
  toggleDialog.close()
}

// confirmDelete removes the selected provider.
async function confirmDelete() {
  if (!deleteDialog.target.value) {
    return
  }

  await adminProviders.deleteProvider(deleteDialog.target.value.id)
  deleteDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-cloud-cog-outline"
          kicker="AI control plane"
          title="Providers"
          copy="Manage upstream LLM provider endpoints, credentials, and runtime availability for proxy traffic."
          stat-label="Enabled providers"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="adminProviders.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminProviders.listError.value }}
        </v-alert>

        <AdminProvidersTable
          :items="adminProviders.providers.value"
          :loading="adminProviders.loading.value"
          :page="adminProviders.page.value"
          :page-size="adminProviders.pageSize.value"
          :sort-by="adminProviders.sortBy.value"
          :sort-dir="adminProviders.sortDir.value"
          :total="adminProviders.total.value"
          @create="openCreateDialog"
          @delete="deleteDialog.open"
          @edit="openEditDialog"
          @refresh="adminProviders.reload"
          @toggle="toggleDialog.open"
          @update:page="adminProviders.setPage"
          @update:page-size="adminProviders.setPageSize"
          @update:sort="adminProviders.setSort"
        />
      </v-col>
    </v-row>

    <AdminProviderDialog
      v-model="providerDialogOpen"
      :loading="adminProviders.saving.value"
      :provider="adminProviders.selectedProvider.value"
      @save="saveProvider"
    />

    <AppConfirmDialog
      v-model="deleteDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Delete provider"
      icon="mdi-delete-alert-outline"
      :loading="adminProviders.saving.value"
      :message="
        deleteDialog.target.value
          ? `Delete provider ${deleteDialog.target.value.name}.`
          : 'Delete this provider.'
      "
      title="Delete provider"
      @cancel="deleteDialog.close"
      @confirm="confirmDelete"
    />

    <AppConfirmDialog
      v-model="toggleDialog.isOpen.value"
      :confirm-color="toggleConfirm.confirmColor.value"
      :confirm-label="toggleConfirm.actionLabel.value"
      :icon="toggleConfirm.icon.value"
      :loading="adminProviders.saving.value"
      :message="toggleConfirm.message.value"
      :title="toggleConfirm.title.value"
      @cancel="toggleDialog.close"
      @confirm="confirmToggleProvider"
    />
  </v-container>
</template>
