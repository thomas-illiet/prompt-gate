<script setup lang="ts">
import type {
  DashboardBreakdown,
  DashboardCostCoverage,
  DashboardMetric,
  DashboardScope,
  UsageWindow,
} from '~/types/user-service'
import { formatDate } from '~/utils/formatters'

definePageMeta({
  icon: 'mdi-monitor-dashboard',
  title: 'Dashboard',
  drawerIndex: 0,
  requiredRoles: ['limited', 'user', 'admin'],
})

const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()
const isAdmin = computed(() => authStore.user?.role === 'admin')
const selectedWindow = shallowRef<UsageWindow>(parseWindow(route.query.window))
const selectedScope = shallowRef<DashboardScope>(
  isAdmin.value && queryValue(route.query.scope) === 'global'
    ? 'global'
    : 'self',
)
const selectedMetric = shallowRef<DashboardMetric>(
  parseMetric(route.query.metric),
)
const selectedBreakdown = shallowRef<DashboardBreakdown>(
  parseBreakdown(route.query.breakdown),
)
const refreshVersion = shallowRef(0)
const dashboardScope = computed<DashboardScope>(() =>
  isAdmin.value ? selectedScope.value : 'self',
)
const overview = useDashboardOverview(dashboardScope, selectedWindow)

const title = computed(() =>
  dashboardScope.value === 'global' ? 'Organization overview' : 'My usage',
)
const subtitle = computed(() =>
  dashboardScope.value === 'global'
    ? 'All signals, one radar. Spot operational risk, then understand how the organization is consuming PromptGate.'
    : 'Your flight recorder, minus the turbulence. Check what needs attention, review consumption, then explore what drove it.',
)
const periodLabel = computed(() => {
  const data = overview.data.value
  if (!data) return 'Loading exact period…'
  return `${formatDate(data.startsAt)} – ${formatDate(data.endsAt)}`
})
const hasActivity = computed(() => {
  const totals = overview.data.value?.totals
  return Boolean(
    totals &&
    (totals.requests > 0 || totals.totalTokens > 0 || totals.toolCalls > 0),
  )
})
const emptyAction = computed(() =>
  authStore.user?.role === 'limited'
    ? {
        label: 'Browse FAQ',
        text: 'No API activity was recorded for this period. Browse the FAQ or contact your administrator to request access.',
        to: '/faq',
      }
    : {
        label: 'Open setup guide',
        text: 'No API activity was recorded for this period. Configure a client, send your first request, or choose another period.',
        to: '/help',
      },
)
const effectiveCostCoverage = computed<DashboardCostCoverage>(() => {
  const data = overview.data.value
  if (data?.costCoverage) return data.costCoverage
  return data?.totals.estimatedCost
    ? { status: 'complete' }
    : { status: 'unavailable' }
})

watch(
  isAdmin,
  (admin) => {
    if (!admin) {
      selectedScope.value = 'self'
    } else if (queryValue(route.query.scope) === 'global') {
      selectedScope.value = 'global'
    }

    if (
      selectedBreakdown.value === 'identities' &&
      (!admin || selectedScope.value !== 'global')
    ) {
      selectedBreakdown.value = 'models'
    }
  },
  { immediate: true },
)

watch(
  () => route.query,
  (query) => {
    selectedWindow.value = parseWindow(query.window)
    selectedMetric.value = parseMetric(query.metric)
    const breakdown = parseBreakdown(query.breakdown)
    selectedBreakdown.value =
      breakdown === 'identities' && dashboardScope.value !== 'global'
        ? 'models'
        : breakdown
    selectedScope.value =
      isAdmin.value && queryValue(query.scope) === 'global' ? 'global' : 'self'
  },
  { deep: true },
)

watch(
  [dashboardScope, selectedWindow, selectedMetric, selectedBreakdown],
  ([scope, window, metric, breakdown]) => {
    const normalizedBreakdown =
      breakdown === 'identities' && scope !== 'global' ? 'models' : breakdown
    if (normalizedBreakdown !== breakdown) {
      selectedBreakdown.value = normalizedBreakdown
      return
    }

    const nextQuery = { ...route.query }
    nextQuery.window = window
    nextQuery.metric = metric
    nextQuery.breakdown = normalizedBreakdown
    if (scope === 'global') nextQuery.scope = 'global'
    else delete nextQuery.scope

    if (
      queryValue(route.query.scope) === (scope === 'global' ? 'global' : '') &&
      queryValue(route.query.window) === window &&
      queryValue(route.query.metric) === metric &&
      queryValue(route.query.breakdown) === normalizedBreakdown
    ) {
      return
    }
    void router.replace({ query: nextQuery })
  },
  { immediate: true },
)

function refreshDashboard() {
  refreshVersion.value += 1
  void overview.reload()
}

function queryValue(value: unknown) {
  return Array.isArray(value) ? String(value[0] ?? '') : String(value ?? '')
}

function parseWindow(value: unknown): UsageWindow {
  const normalized = queryValue(value)
  return normalized === '30d' || normalized === 'all' ? normalized : '7d'
}

function parseMetric(value: unknown): DashboardMetric {
  const normalized = queryValue(value)
  return normalized === 'tokens' || normalized === 'cost'
    ? normalized
    : 'requests'
}

function parseBreakdown(value: unknown): DashboardBreakdown {
  const normalized = queryValue(value)
  if (
    normalized === 'providers' ||
    normalized === 'provider-types' ||
    normalized === 'identities'
  ) {
    return normalized
  }
  return 'models'
}
</script>

<template>
  <v-container fluid class="app-page dashboard-page">
    <DashboardPageHeader
      v-model:scope="selectedScope"
      v-model:window="selectedWindow"
      class="dashboard-page__header"
      :copy="subtitle"
      :is-admin="isAdmin"
      :loading="overview.loading.value"
      :period-label="periodLabel"
      :title="title"
      :updated-at="overview.updatedAt.value"
      @refresh="refreshDashboard"
    />

    <v-alert
      v-if="overview.staleWarning.value"
      class="dashboard-page__notice"
      type="warning"
      variant="tonal"
      rounded="lg"
    >
      {{ overview.staleWarning.value }}
    </v-alert>

    <section class="dashboard-page__section" aria-label="Current status">
      <DashboardCurrentState
        :adoption="overview.data.value?.adoption"
        :refresh-version="refreshVersion"
        :scope="dashboardScope"
      />
    </section>

    <v-alert
      v-if="overview.error.value && !overview.data.value"
      class="dashboard-page__section"
      type="warning"
      variant="tonal"
      rounded="lg"
    >
      <div class="dashboard-page__error">
        <span>{{ overview.error.value }}</span>
        <v-btn
          size="small"
          variant="text"
          prepend-icon="mdi-refresh"
          @click="overview.reload"
        >
          Retry
        </v-btn>
      </div>
    </v-alert>

    <div
      v-else-if="overview.loading.value && !overview.data.value"
      class="dashboard-page__loading dashboard-page__section"
      aria-label="Loading dashboard overview"
    >
      <span v-for="index in 6" :key="index" />
    </div>

    <AppSectionCard
      v-else-if="overview.data.value && !hasActivity"
      class="dashboard-page__section"
      icon="mdi-chart-timeline-variant-shimmer"
      title="Usage on this period"
      subtitle="Everything is ready for the first signal"
    >
      <AppEmptyState
        icon="mdi-radar"
        title="The launchpad is ready"
        :text="emptyAction.text"
      >
        <template #actions>
          <v-btn
            :to="emptyAction.to"
            color="primary"
            prepend-icon="mdi-arrow-right"
            variant="tonal"
          >
            {{ emptyAction.label }}
          </v-btn>
        </template>
      </AppEmptyState>
    </AppSectionCard>

    <template v-else-if="overview.data.value">
      <DashboardPeriodKpis
        class="dashboard-page__section"
        :overview="overview.data.value"
      />

      <DashboardTrend
        v-model="selectedMetric"
        class="dashboard-page__section"
        :cost-coverage="effectiveCostCoverage"
        :daily="overview.data.value.daily"
      />

      <DashboardBreakdownPanel
        v-model="selectedBreakdown"
        class="dashboard-page__section"
        :metric="selectedMetric"
        :refresh-version="refreshVersion"
        :scope="dashboardScope"
        :window="selectedWindow"
      />

      <DashboardTopTools
        class="dashboard-page__section"
        :refresh-version="refreshVersion"
        :scope="dashboardScope"
        :window="selectedWindow"
      />

      <DashboardAnnualActivity
        v-if="dashboardScope === 'self'"
        class="dashboard-page__section"
        :refresh-version="refreshVersion"
      />
    </template>
  </v-container>
</template>

<style scoped>
.dashboard-page {
  display: grid;
  gap: 0;
}

.dashboard-page__header {
  margin-bottom: 18px;
}

.dashboard-page__section,
.dashboard-page__notice {
  margin-bottom: 18px;
}

.dashboard-page__error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dashboard-page__loading {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.dashboard-page__loading span {
  min-height: 124px;
  border: 1px solid rgba(var(--app-shell-border), 0.3);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-border), 0.25);
}

@media (max-width: 959px) {
  .dashboard-page__loading {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 599px) {
  .dashboard-page__loading {
    grid-template-columns: 1fr;
  }

  .dashboard-page__error {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
