<script setup lang="ts">
import AdminActivityAdoptionKpis from '~/components/AdminDashboards/AdminActivityAdoptionKpis.vue'

import type { DashboardBreakdown } from '~/types/user-service'
import { formatDate } from '~/utils/formatters'

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

    <AdminActivityAdoptionKpis
      :adoption="overview.data.value?.adoption"
      :loading="overview.loading.value"
    />

    <DashboardBreakdownPanel
      v-model="filters.breakdown.value"
      :allowed-breakdowns="allowedBreakdowns"
      icon="mdi-account-multiple-outline"
      :metric="filters.metric.value"
      :refresh-version="refreshVersion"
      scope="global"
      subtitle="Compare people and services with one metric"
      title="Identity ranking"
      :window="filters.window.value"
    >
      <template #actions>
        <DashboardMetricSelect
          v-model="filters.metric.value"
          aria-label="Identity ranking metric"
        />
      </template>
    </DashboardBreakdownPanel>

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
</style>
