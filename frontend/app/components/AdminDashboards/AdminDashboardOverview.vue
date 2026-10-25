<script setup lang="ts">
import AdminDashboardThemeCard from '~/components/AdminDashboards/AdminDashboardThemeCard.vue'

import type { DashboardCostCoverage } from '~/types/user-service'
import {
  formatCompactNumber,
  formatCurrencyUsd,
  formatDate,
  formatNumber,
} from '~/utils/formatters'

const selectedWindow = useAdminDashboardWindowQuery()
const refreshVersion = shallowRef(0)
const overview = useDashboardOverview(() => 'global', selectedWindow)
const quotaRisk = useAdminDashboardQuotaRisk(refreshVersion)

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
const costValue = computed(() =>
  costCoverage.value.status === 'unavailable'
    ? '—'
    : formatCurrencyUsd(overview.data.value?.totals.estimatedCost?.totalUsd),
)
const overviewError = computed(() =>
  overview.data.value ? overview.staleWarning.value : overview.error.value,
)

function refresh() {
  refreshVersion.value += 1
  void overview.reload()
}
</script>

<template>
  <v-container fluid class="app-page admin-dashboard-overview">
    <DashboardPageHeader
      v-model:window="selectedWindow"
      copy="A light operational summary. Open a focused dashboard when a signal needs investigation."
      icon="mdi-view-dashboard-outline"
      kicker="Administration · Dashboards"
      :loading="overview.loading.value || quotaRisk.loading.value"
      :period-label="periodLabel"
      title="Overview"
      :updated-at="overview.updatedAt.value"
      @refresh="refresh"
    />

    <section
      class="admin-dashboard-overview__grid"
      aria-label="Specialized dashboards"
    >
      <AdminDashboardThemeCard
        copy="Traffic volume, token consumption, cost, latency, and infrastructure attribution."
        :error="overviewError"
        icon="mdi-chart-line"
        :loading="overview.loading.value && !overview.data.value"
        :stats="[
          {
            label: 'API requests',
            value: formatCompactNumber(overview.data.value?.totals.requests),
          },
          {
            label: 'Tokens',
            value: formatCompactNumber(overview.data.value?.totals.totalTokens),
          },
          { label: 'Estimated cost', value: costValue },
        ]"
        title="Usage & costs"
        :to="{ path: '/admin/usage', query: { window: selectedWindow } }"
      />

      <AdminDashboardThemeCard
        copy="See which people, service accounts, keys, and tools are active across the organization."
        :error="overviewError"
        icon="mdi-account-group-outline"
        :loading="overview.loading.value && !overview.data.value"
        :stats="[
          {
            label: 'Active users',
            value: formatNumber(overview.data.value?.adoption?.activeUsers),
          },
          {
            label: 'Active services',
            value: formatNumber(
              overview.data.value?.adoption?.activeServiceAccounts,
            ),
          },
          {
            label: 'Active keys',
            value: formatNumber(
              overview.data.value?.adoption?.activeVirtualKeys,
            ),
          },
        ]"
        title="Activity & adoption"
        :to="{ path: '/admin/activity', query: { window: selectedWindow } }"
      />

      <AdminDashboardThemeCard
        copy="Inspect live 5-hour and 7-day projections alongside credit and reset reconciliation."
        :error="quotaRisk.error.value"
        icon="mdi-timer-refresh-outline"
        :loading="quotaRisk.loading.value && !quotaRisk.data.value"
        :stats="[
          {
            label: 'At risk',
            value: formatNumber(quotaRisk.data.value?.atRiskCount),
          },
          {
            label: 'Exhausted',
            value: formatNumber(quotaRisk.data.value?.exhaustedCount),
          },
          {
            label: 'Top risks shown',
            value: formatNumber(quotaRisk.data.value?.items.length),
          },
        ]"
        title="Quotas & resets"
        to="/admin/quota-resets"
      />
    </section>
  </v-container>
</template>

<style scoped>
.admin-dashboard-overview {
  display: grid;
  gap: 18px;
}

.admin-dashboard-overview__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

@media (max-width: 1199px) {
  .admin-dashboard-overview__grid {
    grid-template-columns: 1fr;
  }
}
</style>
