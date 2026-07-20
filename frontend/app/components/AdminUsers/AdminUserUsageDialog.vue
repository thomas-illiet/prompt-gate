<script setup lang="ts">
import AdminUserUsageOverview from '~/components/AdminUsers/AdminUserUsageOverview.vue'
import type {
  DashboardOverviewResponse,
  UsageWindow,
} from '~/types/user-service'
import type { AdminUser } from '~/types/users'
import { adminUserPath } from '~/utils/api-paths'

const props = defineProps<{
  user: AdminUser
}>()

const isOpen = defineModel<boolean>({ default: false })
const selectedWindow = shallowRef<UsageWindow>('7d')
const endpoint = computed(() => adminUserPath(props.user.id, 'statistics'))
const widget = useDashboardWidget<DashboardOverviewResponse>(
  endpoint,
  selectedWindow,
)
const title = computed(() => `${displayUser(props.user)} usage statistics`)

// displayUser returns the most readable user identifier for the dialog title.
function displayUser(user: AdminUser) {
  return user.name || user.preferredUsername || user.email
}
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    content-class="admin-user-usage-dialog__body"
    icon="mdi-chart-box-outline"
    max-width="1080"
    subtitle="Usage attributed to this user across the selected period."
    :title="title"
  >
    <div class="admin-user-usage-dialog__toolbar">
      <DashboardTimeRangeSelect v-model="selectedWindow" />
      <v-btn
        aria-label="Refresh usage statistics"
        color="primary"
        :loading="widget.loading.value"
        prepend-icon="mdi-refresh"
        rounded="lg"
        variant="outlined"
        @click="widget.reload"
      >
        Refresh
      </v-btn>
    </div>

    <AdminUserUsageOverview
      :data="widget.data.value"
      :error="widget.error.value"
      :loading="widget.loading.value"
      @retry="widget.reload"
    />

    <template #actions>
      <AppDialogCloseButton @click="isOpen = false" />
    </template>
  </AppDialogCard>
</template>

<style scoped>
:deep(.admin-user-usage-dialog__body) {
  display: grid;
  gap: 18px;
}

.admin-user-usage-dialog__toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
}

@media (max-width: 600px) {
  .admin-user-usage-dialog__toolbar {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 46px;
    gap: 8px;
  }

  .admin-user-usage-dialog__toolbar :deep(.dashboard-time-range-select) {
    min-width: 0;
    width: 100%;
  }

  .admin-user-usage-dialog__toolbar
    :deep(.dashboard-time-range-select__control) {
    flex: 0 1 clamp(116px, 38vw, 150px);
    min-width: 0;
    width: clamp(116px, 38vw, 150px);
  }

  .admin-user-usage-dialog__toolbar :deep(.v-btn) {
    min-width: 46px;
    width: 46px;
    height: 46px;
    padding-inline: 0;
  }

  .admin-user-usage-dialog__toolbar :deep(.v-btn__prepend) {
    margin-inline: 0;
  }

  .admin-user-usage-dialog__toolbar :deep(.v-btn__content) {
    display: none;
  }
}
</style>
