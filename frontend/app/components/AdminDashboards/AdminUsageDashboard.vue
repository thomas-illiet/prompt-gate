<script setup lang="ts">
import type {
  DashboardBreakdown,
  DashboardCostCoverage,
  DashboardKpiMetric,
} from '~/types/user-service'
import { formatDate } from '~/utils/formatters'

const allowedBreakdowns: DashboardBreakdown[] = [
  'models',
  'providers',
  'provider-types',
]
const visibleMetrics: DashboardKpiMetric[] = [
  'requests',
  'tokens',
  'cost',
  'tokens-per-request',
  'latency',
]
const filters = useAdminDashboardAnalysisQuery(allowedBreakdowns, 'models')
const refreshVersion = shallowRef(0)
const overview = useDashboardOverview(() => 'global', filters.window)

const periodLabel = computed(() => {
  const data = overview.data.value
  return data
    ? `${formatDate(data.startsAt)} – ${formatDate(data.endsAt)}`
    : 'Loading exact period…'
})
const costCoverage = computed<DashboardCostCoverage>(() => {
  const data = overview.data.value
  if (data?.costCoverage) return data.costCoverage
  return data?.totals.estimatedCost
    ? { status: 'complete' }
    : { status: 'unavailable' }
})

function refresh() {
  refreshVersion.value += 1
  void overview.reload()
}
</script>

<template>
  <v-container fluid class="app-page admin-usage-dashboard">
    <DashboardPageHeader
      v-model:window="filters.window.value"
      copy="Analyze organization-wide traffic, cost, latency, and the providers serving it."
      icon="mdi-chart-line"
      kicker="Administration · Dashboards"
      :loading="overview.loading.value"
      :period-label="periodLabel"
      title="Usage & costs"
      :updated-at="overview.updatedAt.value"
      @refresh="refresh"
    />

    <v-alert
      v-if="overview.staleWarning.value"
      type="warning"
      variant="tonal"
      rounded="lg"
    >
      {{ overview.staleWarning.value }}
    </v-alert>

    <v-alert
      v-if="overview.error.value && !overview.data.value"
      type="warning"
      variant="tonal"
      rounded="lg"
    >
      <div class="admin-usage-dashboard__error">
        <span>{{ overview.error.value }}</span>
        <v-btn
          prepend-icon="mdi-refresh"
          variant="text"
          @click="overview.reload"
        >
          Retry
        </v-btn>
      </div>
    </v-alert>

    <div
      v-else-if="overview.loading.value && !overview.data.value"
      class="admin-usage-dashboard__loading"
      aria-label="Loading usage dashboard"
    >
      <span v-for="index in 5" :key="index" />
    </div>

    <template v-else-if="overview.data.value">
      <DashboardPeriodKpis
        :overview="overview.data.value"
        :visible-metrics="visibleMetrics"
      />
      <DashboardTrend
        v-model="filters.metric.value"
        :cost-coverage="costCoverage"
        :daily="overview.data.value.daily"
      />
      <DashboardBreakdownPanel
        v-model="filters.breakdown.value"
        :allowed-breakdowns="allowedBreakdowns"
        :metric="filters.metric.value"
        :refresh-version="refreshVersion"
        scope="global"
        :window="filters.window.value"
      />
    </template>
  </v-container>
</template>

<style scoped>
.admin-usage-dashboard {
  display: grid;
  gap: 18px;
}

.admin-usage-dashboard__error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.admin-usage-dashboard__loading {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.admin-usage-dashboard__loading span {
  min-height: 120px;
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-border), 0.3);
}
</style>
