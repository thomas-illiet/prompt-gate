<script setup lang="ts">
import type {
  DashboardBreakdown,
  DashboardBreakdownResponse,
  DashboardMetric,
  DashboardScope,
  UsageBreakdown,
  UsageWindow,
} from '~/types/user-service'
import { promptGateChartColors } from '~/theme/palette'
import { formatCurrencyUsd, formatNumber } from '~/utils/formatters'

const props = withDefaults(
  defineProps<{
    allowedBreakdowns?: DashboardBreakdown[]
    icon?: string
    metric: DashboardMetric
    refreshVersion: number
    scope: DashboardScope
    subtitle?: string
    title?: string
    window: UsageWindow
  }>(),
  {
    allowedBreakdowns: undefined,
    icon: 'mdi-chart-bar',
    title: 'What explains it',
  },
)

defineSlots<{
  actions?(): unknown
}>()

const activeTab = defineModel<DashboardBreakdown>({ required: true })
const apiFetch = useApiFetch()
const items = shallowRef<UsageBreakdown[]>([])
const loading = shallowRef(false)
const error = shallowRef<string | null>(null)
const reducedMotion = shallowRef(false)
let controller: AbortController | null = null
let requestVersion = 0

const baseTabs: { icon: string; label: string; value: DashboardBreakdown }[] = [
  { icon: 'mdi-cube-outline', label: 'Models', value: 'models' },
  { icon: 'mdi-cloud-outline', label: 'Providers', value: 'providers' },
  {
    icon: 'mdi-shape-outline',
    label: 'Provider types',
    value: 'provider-types',
  },
]

const tabs = computed(() => {
  const available =
    props.scope === 'global'
      ? [
          ...baseTabs,
          {
            icon: 'mdi-account-multiple-outline',
            label: 'Identities',
            value: 'identities' as const,
          },
        ]
      : baseTabs
  const allowedBreakdowns = props.allowedBreakdowns
  if (!allowedBreakdowns) return available
  return available.filter((tab) => allowedBreakdowns.includes(tab.value))
})
const selectedTab = computed(() =>
  tabs.value.some((tab) => tab.value === activeTab.value)
    ? activeTab.value
    : (tabs.value[0]?.value ?? 'models'),
)

const endpoint = computed(() => {
  const prefix =
    props.scope === 'global'
      ? '/api/v1/admin/dashboard'
      : '/api/v1/me/dashboard'
  const suffix =
    selectedTab.value === 'identities'
      ? 'top-identities'
      : selectedTab.value === 'provider-types'
        ? 'top-provider-types'
        : selectedTab.value === 'providers'
          ? 'top-provider-names'
          : 'top-models'
  return `${prefix}/${suffix}`
})

const currentKey = computed(
  () => `${props.scope}:${props.window}:${props.metric}:${selectedTab.value}`,
)

const rankedItems = computed(() =>
  [...items.value].sort((left, right) => {
    const leftValue = metricValue(left)
    const rightValue = metricValue(right)

    if (leftValue == null && rightValue == null) {
      return left.name.localeCompare(right.name)
    }
    if (leftValue == null) return 1
    if (rightValue == null) return -1
    if (rightValue !== leftValue) return rightValue - leftValue
    return left.name.localeCompare(right.name)
  }),
)
const leader = computed(() => rankedItems.value[0] ?? null)
const summary = computed(() => {
  if (!leader.value) {
    return `No ${tabLabel.value.toLowerCase()} data for this period.`
  }
  return `${leader.value.name} leads with ${formatMetric(
    metricValue(leader.value),
  )}.`
})

const tabLabel = computed(
  () =>
    tabs.value.find((tab) => tab.value === selectedTab.value)?.label ??
    'Breakdown',
)
const cardSubtitle = computed(
  () =>
    props.subtitle ??
    `${metricLabel(props.metric)} by ${tabLabel.value.toLowerCase()}`,
)

const option = computed<ECOption>(() => ({
  animation: !reducedMotion.value,
  animationDelay: (index: number) => index * 45,
  animationDuration: 480,
  backgroundColor: 'transparent',
  color: [...promptGateChartColors],
  grid: {
    bottom: 12,
    containLabel: true,
    left: 8,
    right: 24,
    top: 12,
  },
  tooltip: {
    confine: true,
    trigger: 'axis',
    axisPointer: { type: 'shadow' },
    valueFormatter: (value: unknown) =>
      typeof value === 'number' ? formatMetric(value) : '—',
  },
  xAxis: { type: 'value', axisLabel: { hideOverlap: true } },
  yAxis: {
    type: 'category',
    inverse: true,
    data: rankedItems.value.map((item) => item.name),
    axisLabel: {
      ellipsis: '…',
      overflow: 'truncate',
      width: 120,
    },
  },
  series: [
    {
      type: 'bar',
      name: metricLabel(props.metric),
      barMaxWidth: 26,
      data: rankedItems.value.map((item) => metricValue(item)),
      itemStyle: { borderRadius: [0, 6, 6, 0] },
    },
  ],
}))

watch(
  tabs,
  (availableTabs) => {
    if (!availableTabs.some((tab) => tab.value === activeTab.value)) {
      activeTab.value = 'models'
    }
  },
  { immediate: true },
)

watch(
  [currentKey, () => props.refreshVersion],
  ([key], previous) => {
    if (!previous || key !== previous[0]) {
      items.value = []
    }
    void loadBreakdown(key)
  },
  { immediate: true },
)

onMounted(() => {
  reducedMotion.value =
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
})

onBeforeUnmount(() => {
  requestVersion += 1
  controller?.abort()
})

async function loadBreakdown(requestedKey = currentKey.value) {
  const version = ++requestVersion
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = null

  try {
    const response = await apiFetch<DashboardBreakdownResponse>(
      `${endpoint.value}?window=${props.window}&metric=${props.metric}`,
      { signal: controller.signal },
    )
    if (version === requestVersion && requestedKey === currentKey.value) {
      items.value = response.items
    }
  } catch (fetchError) {
    if (
      version === requestVersion &&
      !(fetchError instanceof Error && fetchError.name === 'AbortError')
    ) {
      error.value = 'Unable to load this breakdown.'
    }
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

function metricValue(item: UsageBreakdown) {
  if (props.metric === 'requests') return item.requests
  if (props.metric === 'cost') return item.estimatedCost?.totalUsd ?? null
  return item.totalTokens
}

function metricLabel(selectedMetric: DashboardMetric) {
  if (selectedMetric === 'requests') return 'API requests'
  if (selectedMetric === 'cost') return 'Estimated cost'
  return 'Tokens'
}

function formatMetric(value: number | null) {
  if (value == null) return '—'
  return props.metric === 'cost'
    ? formatCurrencyUsd(value)
    : formatNumber(value)
}
</script>

<template>
  <AppSectionCard
    :icon="props.icon"
    :title="props.title"
    :subtitle="cardSubtitle"
  >
    <template v-if="$slots.actions || tabs.length > 1" #actions>
      <slot name="actions">
        <v-btn-toggle
          v-model="activeTab"
          aria-label="Usage breakdown"
          class="dashboard-breakdown-panel__selector"
          color="primary"
          density="compact"
          mandatory
          variant="outlined"
        >
          <v-btn
            v-for="tab in tabs"
            :key="tab.value"
            :aria-label="`Show ${tab.label.toLowerCase()} breakdown`"
            :value="tab.value"
            size="small"
          >
            <v-icon :icon="tab.icon" start />
            {{ tab.label }}
          </v-btn>
        </v-btn-toggle>
      </slot>
    </template>

    <div class="dashboard-breakdown-panel">
      <p class="dashboard-breakdown-panel__summary" aria-live="polite">
        {{ summary }}
      </p>

      <div
        v-if="loading && items.length === 0"
        class="dashboard-breakdown-panel__loading"
        aria-label="Loading usage breakdown"
      >
        <span v-for="index in 5" :key="index" />
      </div>

      <v-alert v-else-if="error" type="warning" variant="tonal" rounded="lg">
        <div class="dashboard-breakdown-panel__alert">
          <span>{{ error }}</span>
          <v-btn
            size="small"
            variant="text"
            prepend-icon="mdi-refresh"
            @click="loadBreakdown()"
          >
            Retry
          </v-btn>
        </div>
      </v-alert>

      <p
        v-else-if="items.length === 0"
        class="dashboard-breakdown-panel__empty"
      >
        Traffic has no {{ tabLabel.toLowerCase() }} attribution for this period.
      </p>

      <template v-else>
        <v-chart
          class="dashboard-breakdown-panel__chart"
          :option="option"
          autoresize
          role="img"
          :aria-label="`${tabLabel} ranked by ${metricLabel(props.metric)}. ${summary}`"
        />

        <details class="dashboard-breakdown-panel__data">
          <summary>View ranked data</summary>
          <div class="dashboard-breakdown-panel__table-scroll">
            <table>
              <caption>
                {{
                  tabLabel
                }}
                by
                {{
                  metricLabel(props.metric).toLowerCase()
                }}
              </caption>
              <thead>
                <tr>
                  <th scope="col">Name</th>
                  <th scope="col">API requests</th>
                  <th scope="col">Tokens</th>
                  <th scope="col">Estimated cost</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in rankedItems" :key="item.name">
                  <th scope="row">{{ item.name }}</th>
                  <td>{{ formatNumber(item.requests) }}</td>
                  <td>{{ formatNumber(item.totalTokens) }}</td>
                  <td>
                    {{
                      item.estimatedCost
                        ? formatCurrencyUsd(item.estimatedCost.totalUsd)
                        : '—'
                    }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </details>
      </template>
    </div>
  </AppSectionCard>
</template>

<style scoped>
.dashboard-breakdown-panel {
  display: grid;
  gap: 10px;
  padding: 0 20px 20px;
}

.dashboard-breakdown-panel__summary,
.dashboard-breakdown-panel__empty {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.88rem;
}

.dashboard-breakdown-panel__empty {
  display: grid;
  min-height: 220px;
  place-items: center;
  text-align: center;
}

.dashboard-breakdown-panel__chart,
.dashboard-breakdown-panel__loading {
  width: 100%;
  height: 310px;
}

.dashboard-breakdown-panel__loading {
  display: grid;
  align-content: center;
  gap: 15px;
}

.dashboard-breakdown-panel__loading span {
  height: 22px;
  border-radius: 5px;
  background: rgba(var(--app-shell-border), 0.34);
}

.dashboard-breakdown-panel__loading span:nth-child(2) {
  width: 82%;
}
.dashboard-breakdown-panel__loading span:nth-child(3) {
  width: 68%;
}
.dashboard-breakdown-panel__loading span:nth-child(4) {
  width: 52%;
}
.dashboard-breakdown-panel__loading span:nth-child(5) {
  width: 38%;
}

.dashboard-breakdown-panel__alert {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dashboard-breakdown-panel__data {
  border-top: 1px solid rgba(var(--app-shell-border), 0.45);
  padding-top: 10px;
}

.dashboard-breakdown-panel__data summary {
  width: fit-content;
  cursor: pointer;
  color: rgb(var(--v-theme-primary));
  font-size: 0.86rem;
  font-weight: 700;
}

.dashboard-breakdown-panel__table-scroll {
  max-height: 300px;
  margin-top: 10px;
  overflow: auto;
}

.dashboard-breakdown-panel table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.84rem;
}

.dashboard-breakdown-panel caption {
  padding: 8px 0;
  text-align: left;
  font-weight: 700;
}

.dashboard-breakdown-panel th,
.dashboard-breakdown-panel td {
  padding: 8px 10px;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.36);
  text-align: left;
}

@media (max-width: 599px) {
  .dashboard-breakdown-panel__selector {
    width: 100%;
    max-width: 100%;
    justify-content: flex-start;
    overflow-x: auto;
  }

  .dashboard-breakdown-panel {
    padding: 0 12px 16px;
  }

  .dashboard-breakdown-panel__chart {
    height: 270px;
  }
}
</style>
