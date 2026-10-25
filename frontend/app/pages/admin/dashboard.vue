<script setup lang="ts">
import AdminDashboardOverview from '~/components/AdminDashboards/AdminDashboardOverview.vue'
import type {
  DashboardBreakdown,
  DashboardMetric,
  UsageWindow,
} from '~/types/user-service'
import { dashboardQueryValue } from '~/composables/useAdminDashboardRouteQuery'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Overview',
  icon: 'mdi-view-dashboard-outline',
  drawerIndex: 0,
  drawerSection: 'Dashboards',
})

const route = useRoute()
const router = useRouter()
const legacyQuery = computed(
  () =>
    Object.hasOwn(route.query, 'metric') ||
    Object.hasOwn(route.query, 'breakdown'),
)

function validLegacyQuery() {
  const query: Record<string, string> = {}
  const window = dashboardQueryValue(route.query.window) as UsageWindow
  const metric = dashboardQueryValue(route.query.metric) as DashboardMetric
  const breakdown = dashboardQueryValue(
    route.query.breakdown,
  ) as DashboardBreakdown

  if (['7d', '30d', 'all'].includes(window)) query.window = window
  if (['requests', 'tokens', 'cost'].includes(metric)) query.metric = metric
  if (['models', 'providers', 'provider-types'].includes(breakdown)) {
    query.breakdown = breakdown
  }
  return query
}

watch(
  () => route.query,
  () => {
    if (legacyQuery.value) {
      void router.replace({ path: '/admin/usage', query: validLegacyQuery() })
    }
  },
  { deep: true, immediate: true },
)
</script>

<template>
  <AdminDashboardOverview v-if="!legacyQuery" />
</template>
