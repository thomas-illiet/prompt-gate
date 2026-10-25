<script setup lang="ts">
import type { DashboardBreakdown } from '~/types/user-service'
import { formatDate, formatNumber } from '~/utils/formatters'

const allowedBreakdowns: DashboardBreakdown[] = ['identities']
const filters = useAdminDashboardAnalysisQuery(allowedBreakdowns, 'identities')
const refreshVersion = shallowRef(0)
const overview = useDashboardOverview(() => 'global', filters.window)

const periodLabel = computed(() => {
  const data = overview.data.value
  return data
    ? `${formatDate(data.startsAt)} – ${formatDate(data.endsAt)}`
    : 'Loading exact period…'
})

function refresh() {
  refreshVersion.value += 1
  void overview.reload()
}
</script>

<template>
  <v-container fluid class="app-page admin-activity-dashboard">
    <DashboardPageHeader
      v-model:window="filters.window.value"
      copy="Follow adoption by identity and see which tools are most frequently invoked."
      icon="mdi-account-group-outline"
      kicker="Administration · Dashboards"
      :loading="overview.loading.value"
      :period-label="periodLabel"
      title="Activity & adoption"
      :updated-at="overview.updatedAt.value"
      @refresh="refresh"
    />

    <v-alert
      v-if="overview.staleWarning.value || overview.error.value"
      type="warning"
      variant="tonal"
      rounded="lg"
    >
      {{ overview.staleWarning.value || overview.error.value }}
    </v-alert>

    <section class="admin-activity-dashboard__adoption" aria-label="Adoption">
      <DashboardKpiCard
        caption="Used this period"
        color="info"
        :error="overview.error.value"
        :formatter="formatNumber"
        icon="mdi-account-check-outline"
        :loading="overview.loading.value"
        title="Active users"
        :value="overview.data.value?.adoption?.activeUsers"
        @retry="overview.reload"
      />
      <DashboardKpiCard
        caption="Used this period"
        color="secondary"
        :error="overview.error.value"
        :formatter="formatNumber"
        icon="mdi-account-cog-outline"
        :loading="overview.loading.value"
        title="Active service accounts"
        :value="overview.data.value?.adoption?.activeServiceAccounts"
        @retry="overview.reload"
      />
      <DashboardKpiCard
        caption="Used this period"
        color="primary"
        :error="overview.error.value"
        :formatter="formatNumber"
        icon="mdi-key-chain"
        :loading="overview.loading.value"
        title="Active virtual keys"
        :value="overview.data.value?.adoption?.activeVirtualKeys"
        @retry="overview.reload"
      />
    </section>

    <div class="admin-activity-dashboard__ranking-control">
      <div>
        <strong>Identity ranking</strong>
        <span>Compare people and services with one metric.</span>
      </div>
      <DashboardMetricSelect
        v-model="filters.metric.value"
        aria-label="Identity ranking metric"
      />
    </div>

    <DashboardBreakdownPanel
      v-model="filters.breakdown.value"
      :allowed-breakdowns="allowedBreakdowns"
      :metric="filters.metric.value"
      :refresh-version="refreshVersion"
      scope="global"
      :window="filters.window.value"
    />

    <DashboardTopTools
      :refresh-version="refreshVersion"
      scope="global"
      :window="filters.window.value"
    />
  </v-container>
</template>

<style scoped>
.admin-activity-dashboard {
  display: grid;
  gap: 18px;
}

.admin-activity-dashboard__adoption {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.admin-activity-dashboard__ranking-control {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface), 0.92);
}

.admin-activity-dashboard__ranking-control > div {
  display: grid;
  gap: 3px;
}

.admin-activity-dashboard__ranking-control span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.84rem;
}

@media (max-width: 799px) {
  .admin-activity-dashboard__adoption {
    grid-template-columns: 1fr;
  }

  .admin-activity-dashboard__ranking-control {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
