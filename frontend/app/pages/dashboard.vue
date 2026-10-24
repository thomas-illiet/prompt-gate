<script setup lang="ts">
import type {
  DashboardBreakdown,
  DashboardCostCoverage,
  DashboardMetric,
  DashboardScope,
  UsageWindow,
} from '~/types/user-service'
import { formatDate, formatDateTime } from '~/utils/formatters'

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
    <header class="dashboard-page__header">
      <div class="dashboard-page__heading">
        <p class="dashboard-page__kicker">Usage cockpit</p>
        <h1>{{ title }}</h1>
        <p>{{ subtitle }}</p>
        <div class="dashboard-page__period" aria-live="polite">
          <v-icon icon="mdi-calendar-range" size="17" />
          <span>{{ periodLabel }}</span>
          <span v-if="overview.updatedAt.value" aria-hidden="true">·</span>
          <span v-if="overview.updatedAt.value">
            Updated {{ formatDateTime(overview.updatedAt.value) }}
          </span>
        </div>
      </div>

      <div class="dashboard-page__visual" aria-hidden="true">
        <AppSignalVisual name="radar" />
      </div>

      <div class="dashboard-page__controls" aria-label="Dashboard controls">
        <DashboardScopeSelect v-if="isAdmin" v-model="selectedScope" />
        <DashboardTimeRangeSelect v-model="selectedWindow" />
        <v-btn
          class="dashboard-page__refresh"
          color="primary"
          prepend-icon="mdi-refresh"
          variant="outlined"
          rounded="lg"
          :loading="overview.loading.value"
          aria-label="Refresh all dashboard data"
          @click="refreshDashboard"
        >
          Refresh
        </v-btn>
      </div>
    </header>

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
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 104px;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
  padding: 16px 18px;
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: var(--app-card-radius);
  background:
    radial-gradient(
      circle at 72% 8%,
      rgba(var(--app-accent-coral), 0.12),
      transparent 24%
    ),
    linear-gradient(
      90deg,
      rgba(var(--v-theme-primary), 0.08) 0,
      rgba(var(--v-theme-primary), 0) 5px
    ),
    linear-gradient(
      180deg,
      rgba(var(--app-shell-surface-strong), 0.98) 0%,
      rgba(var(--app-shell-surface), 0.98) 58%,
      rgba(var(--app-shell-surface-muted), 0.82) 100%
    );
  box-shadow: var(--app-card-shadow-soft);
}

.dashboard-page__heading {
  display: grid;
  max-width: 48rem;
  gap: 5px;
}

.dashboard-page__kicker,
.dashboard-page__heading h1,
.dashboard-page__heading > p {
  margin: 0;
}

.dashboard-page__kicker {
  color: rgb(var(--app-shell-text-muted));
  font-family: var(--app-font-mono);
  font-size: 0.76rem;
  font-weight: 750;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.dashboard-page__heading h1 {
  font-family: var(--app-font-display);
  font-size: 1.55rem;
  font-weight: 800;
}

.dashboard-page__heading > p {
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.5;
}

.dashboard-page__period {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 5px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
  font-weight: 650;
}

.dashboard-page__visual {
  position: relative;
  display: grid;
  width: 104px;
  height: 104px;
  place-items: center;
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.6);
  border-radius: calc(var(--app-card-radius) - 2px);
  background:
    radial-gradient(
      circle at 70% 20%,
      rgba(var(--app-accent-coral), 0.12),
      transparent 42%
    ),
    rgba(var(--v-theme-primary), 0.055);
  box-shadow: var(--app-card-shadow-soft);
  padding: 10px;
}

.dashboard-page__controls {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  grid-column: 1 / -1;
  justify-content: flex-start;
  gap: 10px;
}

.dashboard-page__refresh {
  min-height: 46px;
  font-weight: 700;
  text-transform: none;
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
  .dashboard-page__header {
    grid-template-columns: minmax(0, 1fr) 88px;
  }

  .dashboard-page__visual {
    width: 88px;
    height: 88px;
  }

  .dashboard-page__loading {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 599px) {
  .dashboard-page__header {
    grid-template-columns: minmax(0, 1fr);
    padding: 15px;
  }

  .dashboard-page__visual {
    display: none;
  }

  .dashboard-page__controls {
    align-items: stretch;
    flex-direction: column;
  }

  .dashboard-page__refresh {
    width: 100%;
  }

  .dashboard-page__loading {
    grid-template-columns: 1fr;
  }

  .dashboard-page__error {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
