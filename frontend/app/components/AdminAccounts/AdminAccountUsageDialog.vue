<script setup lang="ts">
import AdminAccountUsageOverview from '~/components/AdminAccounts/AdminAccountUsageOverview.vue'
import type {
  DashboardOverviewResponse,
  UsageWindow,
} from '~/types/user-service'

const props = defineProps<{
  costCaption: string
  emptyUsageText: string
  endpoint: string
  subtitle: string
  title: string
}>()

const isOpen = defineModel<boolean>({ default: false })
const selectedWindow = shallowRef<UsageWindow>('7d')
const widget = useDashboardWidget<DashboardOverviewResponse>(
  () => props.endpoint,
  selectedWindow,
)
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    content-class="admin-account-usage-dialog__body"
    icon="mdi-chart-box-outline"
    max-width="1080"
    :subtitle="props.subtitle"
    :title="props.title"
  >
    <div class="admin-account-usage-dialog__toolbar">
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

    <AdminAccountUsageOverview
      :cost-caption="props.costCaption"
      :data="widget.data.value"
      :empty-usage-text="props.emptyUsageText"
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
:deep(.admin-account-usage-dialog__body) {
  display: grid;
  gap: 18px;
}

.admin-account-usage-dialog__toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
}

@media (max-width: 600px) {
  .admin-account-usage-dialog__toolbar {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 46px;
    gap: 8px;
  }

  .admin-account-usage-dialog__toolbar :deep(.dashboard-time-range-select) {
    min-width: 0;
    width: 100%;
  }

  .admin-account-usage-dialog__toolbar
    :deep(.dashboard-time-range-select__control) {
    flex: 0 1 clamp(116px, 38vw, 150px);
    min-width: 0;
    width: clamp(116px, 38vw, 150px);
  }

  .admin-account-usage-dialog__toolbar :deep(.v-btn) {
    min-width: 46px;
    width: 46px;
    height: 46px;
    padding-inline: 0;
  }

  .admin-account-usage-dialog__toolbar :deep(.v-btn__prepend) {
    margin-inline: 0;
  }

  .admin-account-usage-dialog__toolbar :deep(.v-btn__content) {
    display: none;
  }
}
</style>
