<script setup lang="ts">
import AdminNotificationDetailsDialog from '~/components/AdminNotifications/AdminNotificationDetailsDialog.vue'
import AdminNotificationHistory from '~/components/AdminNotifications/AdminNotificationHistory.vue'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Email delivery',
  icon: 'mdi-email-fast-outline',
  drawerIndex: 13,
  drawerSection: 'Operations',
})

const notifications = useAdminNotifications()
const detailsOpen = shallowRef(false)
const detailsLoading = shallowRef(false)

onMounted(notifications.loadHistory)

watch([notifications.search, notifications.kind, notifications.status], () =>
  notifications.applyFilters(),
)

async function openDetails(id: string) {
  detailsOpen.value = true
  detailsLoading.value = true
  try {
    await notifications.loadMessage(id)
  } finally {
    detailsLoading.value = false
  }
}

function updatePage(value: number) {
  notifications.page.value = value
  void notifications.loadHistory()
}

function updatePageSize(value: number) {
  notifications.pageSize.value = value
  void notifications.loadHistory()
}
</script>

<template>
  <v-container fluid class="app-page">
    <AppPageHeader
      density="compact"
      icon="mdi-email-fast-outline"
      kicker="Communications"
      title="Email delivery"
      copy="Follow queued, delivered, and failed emails while retaining a complete operational audit trail."
      visual="ledger"
      stat-label="Messages"
      :stat-busy="notifications.loading.value"
      :stat-value="String(notifications.total.value)"
    />
    <v-alert
      v-if="notifications.error.value"
      class="my-4"
      type="warning"
      variant="tonal"
    >
      {{ notifications.error.value }}
    </v-alert>
    <AdminNotificationHistory
      class="mt-6"
      :items="notifications.items.value"
      :kind="notifications.kind.value"
      :loading="notifications.loading.value"
      :page="notifications.page.value"
      :page-size="notifications.pageSize.value"
      :search="notifications.search.value"
      :status="notifications.status.value"
      :total="notifications.total.value"
      @details="openDetails($event.id)"
      @refresh="notifications.loadHistory"
      @update:kind="notifications.kind.value = $event"
      @update:page="updatePage"
      @update:page-size="updatePageSize"
      @update:search="notifications.search.value = $event"
      @update:status="notifications.status.value = $event"
    />
    <AdminNotificationDetailsDialog
      v-model="detailsOpen"
      :loading="detailsLoading"
      :message="notifications.selectedMessage.value"
      :retrying="notifications.saving.value"
      @retry="notifications.retryFailed"
    />
  </v-container>
</template>
