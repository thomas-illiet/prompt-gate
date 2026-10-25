<script setup lang="ts">
import AdminQuotaResetEventsSection from '~/components/AdminQuotaResets/AdminQuotaResetEventsSection.vue'
import AdminQuotaResetKpis from '~/components/AdminQuotaResets/AdminQuotaResetKpis.vue'
import AdminQuotaStatesSection from '~/components/AdminQuotaResets/AdminQuotaStatesSection.vue'
import { formatDateTime } from '~/utils/formatters'

const quotaStates = useAdminQuotaStates()
const resetEvents = useAdminQuotaResetEvents()

const refreshing = computed(
  () => quotaStates.loading.value || resetEvents.loading.value,
)

function refreshAll() {
  void Promise.allSettled([quotaStates.reload(), resetEvents.reload()])
}
</script>

<template>
  <v-container fluid class="app-page admin-quota-resets-dashboard">
    <AppPageHeader
      copy="Monitor live 5-hour and 7-day limits, then review how credits and resets were applied. This dashboard is read-only."
      density="compact"
      icon="mdi-timer-refresh-outline"
      kicker="Administration · Dashboards"
      title="Quotas & resets"
      visual="radar"
    >
      <template #meta>
        <div class="admin-quota-resets-dashboard__meta">
          <v-chip color="primary" label size="small" variant="tonal">
            Read-only
          </v-chip>
          <span v-if="quotaStates.projectionSyncedAt.value">
            Projection snapshot
            {{ formatDateTime(quotaStates.projectionSyncedAt.value) }}
          </span>
          <span v-else>Projection snapshot not available yet</span>
        </div>
      </template>
      <template #footer>
        <div class="admin-quota-resets-dashboard__header-actions">
          <span>No polling. Refresh when you need a new snapshot.</span>
          <v-btn
            aria-label="Refresh quota projections and reset events"
            color="primary"
            prepend-icon="mdi-refresh"
            rounded="lg"
            variant="outlined"
            :loading="refreshing"
            @click="refreshAll"
          >
            Refresh all
          </v-btn>
        </div>
      </template>
    </AppPageHeader>

    <AdminQuotaResetKpis
      :events-available="resetEvents.endsAt.value != null"
      :events-loading="resetEvents.loading.value"
      :events-summary="resetEvents.summary.value"
      :states-available="quotaStates.generatedAt.value != null"
      :states-loading="quotaStates.loading.value"
      :states-summary="quotaStates.summary.value"
    />

    <AdminQuotaStatesSection
      :account-type="quotaStates.accountType.value"
      :error="quotaStates.error.value"
      :items="quotaStates.items.value"
      :loading="quotaStates.loading.value"
      :page="quotaStates.page.value"
      :page-size="quotaStates.pageSize.value"
      :projection-synced-at="quotaStates.projectionSyncedAt.value"
      :search="quotaStates.search.value"
      :sort-by="quotaStates.sortBy.value"
      :sort-dir="quotaStates.sortDir.value"
      :total="quotaStates.total.value"
      :view="quotaStates.view.value"
      @refresh="quotaStates.reload"
      @update:account-type="quotaStates.setAccountType"
      @update:page="quotaStates.setPage"
      @update:page-size="quotaStates.setPageSize"
      @update:search="quotaStates.setSearch"
      @update:sort="quotaStates.setSort"
      @update:view="quotaStates.setView"
    />

    <AdminQuotaResetEventsSection
      :error="resetEvents.error.value"
      :items="resetEvents.items.value"
      :loading="resetEvents.loading.value"
      :page="resetEvents.page.value"
      :page-size="resetEvents.pageSize.value"
      :search="resetEvents.search.value"
      :sort-by="resetEvents.sortBy.value"
      :sort-dir="resetEvents.sortDir.value"
      :status="resetEvents.status.value"
      :total="resetEvents.total.value"
      :type="resetEvents.type.value"
      :window="resetEvents.window.value"
      @refresh="resetEvents.reload"
      @update:page="resetEvents.setPage"
      @update:page-size="resetEvents.setPageSize"
      @update:search="resetEvents.setSearch"
      @update:sort="resetEvents.setSort"
      @update:status="resetEvents.setStatus"
      @update:type="resetEvents.setType"
      @update:window="resetEvents.setWindow"
    />
  </v-container>
</template>

<style scoped>
.admin-quota-resets-dashboard {
  display: grid;
  gap: 18px;
}

.admin-quota-resets-dashboard__meta,
.admin-quota-resets-dashboard__header-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
}

.admin-quota-resets-dashboard__header-actions {
  justify-content: space-between;
  padding: 12px 18px;
}

@media (max-width: 599px) {
  .admin-quota-resets-dashboard__header-actions {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
