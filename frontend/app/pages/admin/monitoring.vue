<script setup lang="ts">
import type {
  MonitoringService,
  MonitoringServicePayload,
} from '~/types/monitoring'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Monitoring',
  icon: 'mdi-heart-pulse',
  drawerIndex: 10,
  drawerSection: 'Observability',
})

const adminMonitoring = useAdminMonitoring()
const serviceDialogOpen = shallowRef(false)
const detailsDialogOpen = shallowRef(false)
const detailsLoading = shallowRef(false)
const deleteDialog = useTargetDialog<MonitoringService>()
const toggleDialog = useTargetDialog<MonitoringService>()
const toggleConfirm = useToggleConfirmDialog(toggleDialog.target, {
  disableIcon: 'mdi-heart-off-outline',
  enableIcon: 'mdi-heart-pulse',
  entityLabel: 'monitoring service',
  fallbackMessage: 'Change this monitoring service status.',
  isActive: (service) => service.enabled,
  name: (service) => service.name,
})

const totalLabel = computed(() => {
  const enabled = adminMonitoring.enabledServicesCount.value
  const degraded = adminMonitoring.degradedServicesCount.value
  return enabled === 1
    ? `${degraded}/1 degraded`
    : `${degraded}/${enabled} degraded`
})

function servicePayload(service: MonitoringService, enabled: boolean) {
  return {
    name: service.name,
    displayName: service.displayName,
    url: service.url,
    expectedStatusCode: service.expectedStatusCode,
    intervalSeconds: service.intervalSeconds,
    enabled,
  }
}

function openCreateDialog() {
  adminMonitoring.selectedService.value = null
  serviceDialogOpen.value = true
}

async function openEditDialog(service: MonitoringService) {
  await adminMonitoring.loadService(service.id)
  serviceDialogOpen.value = true
}

async function openDetailsDialog(service: MonitoringService) {
  adminMonitoring.selectedService.value = service
  detailsDialogOpen.value = true

  detailsLoading.value = true
  try {
    await adminMonitoring.loadService(service.id)
  } catch {
    // Keep showing the row snapshot when the fresh detail request fails.
  } finally {
    detailsLoading.value = false
  }
}

async function saveService(payload: MonitoringServicePayload) {
  if (adminMonitoring.selectedService.value) {
    await adminMonitoring.updateService(
      adminMonitoring.selectedService.value.id,
      payload,
    )
  } else {
    await adminMonitoring.createService(payload)
  }

  serviceDialogOpen.value = false
}

async function checkService(service: MonitoringService) {
  await adminMonitoring.checkService(service.id)
}

async function confirmToggleService() {
  if (!toggleDialog.target.value) {
    return
  }

  const service = toggleDialog.target.value
  await adminMonitoring.updateService(
    service.id,
    servicePayload(service, !service.enabled),
  )
  toggleDialog.close()
}

async function confirmDelete() {
  if (!deleteDialog.target.value) {
    return
  }

  await adminMonitoring.deleteService(deleteDialog.target.value.id)
  deleteDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-heart-pulse"
          kicker="Operations"
          title="Monitoring"
          copy="Manage HTTP/S service checks that drive user-facing incident banners."
          stat-label="Service disruption"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="adminMonitoring.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminMonitoring.listError.value }}
        </v-alert>

        <AdminMonitoringTable
          :items="adminMonitoring.services.value"
          :loading="adminMonitoring.loading.value"
          :page="adminMonitoring.page.value"
          :page-size="adminMonitoring.pageSize.value"
          :sort-by="adminMonitoring.sortBy.value"
          :sort-dir="adminMonitoring.sortDir.value"
          :total="adminMonitoring.total.value"
          @check="checkService"
          @create="openCreateDialog"
          @delete="deleteDialog.open"
          @details="openDetailsDialog"
          @edit="openEditDialog"
          @refresh="adminMonitoring.reload"
          @toggle="toggleDialog.open"
          @update:page="adminMonitoring.setPage"
          @update:page-size="adminMonitoring.setPageSize"
          @update:sort="adminMonitoring.setSort"
        />
      </v-col>
    </v-row>

    <AdminMonitoringServiceDialog
      v-model="serviceDialogOpen"
      :loading="adminMonitoring.saving.value"
      :service="adminMonitoring.selectedService.value"
      @save="saveService"
    />

    <AdminMonitoringServiceDetailsDialog
      v-model="detailsDialogOpen"
      :loading="detailsLoading"
      :service="adminMonitoring.selectedService.value"
    />

    <AppConfirmDialog
      v-model="deleteDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Delete service"
      icon="mdi-delete-alert-outline"
      :loading="adminMonitoring.saving.value"
      :message="
        deleteDialog.target.value
          ? `Delete monitoring service ${deleteDialog.target.value.name}.`
          : 'Delete this monitoring service.'
      "
      title="Delete monitoring service"
      @cancel="deleteDialog.close"
      @confirm="confirmDelete"
    />

    <AppConfirmDialog
      v-model="toggleDialog.isOpen.value"
      :confirm-color="toggleConfirm.confirmColor.value"
      :confirm-label="toggleConfirm.actionLabel.value"
      :icon="toggleConfirm.icon.value"
      :loading="adminMonitoring.saving.value"
      :message="toggleConfirm.message.value"
      :title="toggleConfirm.title.value"
      @cancel="toggleDialog.close"
      @confirm="confirmToggleService"
    />
  </v-container>
</template>
