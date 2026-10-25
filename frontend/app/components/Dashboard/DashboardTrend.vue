<script setup lang="ts">
import type {
  DailyUsage,
  DashboardCostCoverage,
  DashboardMetric,
} from '~/types/user-service'
import { promptGateChartColors } from '~/theme/palette'
import { downloadDailyUsageCsv } from '~/utils/daily-usage-csv'
import { formatCurrencyUsd, formatNumber } from '~/utils/formatters'

const props = defineProps<{
  costCoverage: DashboardCostCoverage
  daily: DailyUsage[]
}>()

const metric = defineModel<DashboardMetric>({ required: true })
const reducedMotion = shallowRef(false)

const points = computed(() =>
  props.daily.map((day) => ({
    date: day.date,
    value: metricValue(day, metric.value),
  })),
)

const availablePoints = computed(() =>
  points.value.filter(
    (point): point is { date: string; value: number } => point.value != null,
  ),
)

const total = computed(() =>
  availablePoints.value.reduce((sum, point) => sum + point.value, 0),
)

const peak = computed(() =>
  availablePoints.value.reduce<{ date: string; value: number } | null>(
    (current, point) =>
      !current || point.value > current.value ? point : current,
    null,
  ),
)

const summary = computed(() => {
  if (metric.value === 'cost' && props.costCoverage.status === 'unavailable') {
    return 'Estimated cost is unavailable for this period.'
  }
  if (!peak.value)
    return `No ${metricLabel(metric.value).toLowerCase()} recorded.`
  return `${formatMetric(total.value, metric.value)} in total. Peak: ${formatMetric(
    peak.value.value,
    metric.value,
  )} on ${peak.value.date}.`
})

const option = computed<ECOption>(() => ({
  animation: !reducedMotion.value,
  animationDuration: 550,
  backgroundColor: 'transparent',
  color: [...promptGateChartColors],
  grid: {
    bottom: 20,
    containLabel: true,
    left: 12,
    right: 20,
    top: 24,
  },
  tooltip: {
    confine: true,
    trigger: 'axis',
    valueFormatter: (value: unknown) =>
      typeof value === 'number' ? formatMetric(value, metric.value) : '—',
  },
  xAxis: {
    type: 'category',
    data: points.value.map((point) => point.date),
    axisLabel: { hideOverlap: true },
  },
  yAxis: {
    type: 'value',
    name: metricLabel(metric.value),
  },
  series: [
    {
      type: 'line',
      name: metricLabel(metric.value),
      data: points.value.map((point) => point.value),
      connectNulls: false,
      showSymbol: points.value.length <= 14,
      smooth: true,
      areaStyle: { opacity: 0.09 },
      emphasis: { focus: 'series' },
    },
  ],
}))

onMounted(() => {
  reducedMotion.value =
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
})

function metricValue(day: DailyUsage, selectedMetric: DashboardMetric) {
  if (selectedMetric === 'requests') return day.requests
  if (selectedMetric === 'tokens') return day.totalTokens
  return day.estimatedCost?.totalUsd ?? null
}

function metricLabel(selectedMetric: DashboardMetric) {
  if (selectedMetric === 'requests') return 'API requests'
  if (selectedMetric === 'tokens') return 'Tokens'
  return 'Estimated cost'
}

function formatMetric(value: number, selectedMetric: DashboardMetric) {
  if (selectedMetric === 'cost') return formatCurrencyUsd(value)
  return formatNumber(value)
}

function exportDailyData() {
  const firstDate = props.daily[0]?.date
  const lastDate = props.daily[props.daily.length - 1]?.date
  if (!firstDate || !lastDate) return

  downloadDailyUsageCsv(
    props.daily,
    metric.value,
    `promptgate-daily-${metric.value}-${firstDate}-to-${lastDate}.csv`,
  )
}
</script>

<template>
  <AppSectionCard
    icon="mdi-chart-line"
    title="Evolution"
    subtitle="How consumption changed day by day"
  >
    <template #actions>
      <DashboardMetricSelect v-model="metric" aria-label="Trend metric" />
    </template>

    <div class="dashboard-trend">
      <p class="dashboard-trend__summary" aria-live="polite">
        {{ summary }}
      </p>

      <v-chart
        class="dashboard-trend__chart"
        :option="option"
        autoresize
        role="img"
        :aria-label="`${metricLabel(metric)} daily trend. ${summary}`"
      />

      <details class="dashboard-trend__data">
        <summary>View daily data</summary>
        <div class="dashboard-trend__data-actions">
          <v-btn
            data-test="export-daily-data"
            color="primary"
            prepend-icon="mdi-download-outline"
            rounded="xl"
            size="small"
            type="button"
            variant="tonal"
            @click="exportDailyData"
          >
            Export CSV
          </v-btn>
        </div>
        <div class="dashboard-trend__table-scroll">
          <table>
            <caption>
              Daily
              {{
                metricLabel(metric).toLowerCase()
              }}
            </caption>
            <thead>
              <tr>
                <th scope="col">Date</th>
                <th scope="col">{{ metricLabel(metric) }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="point in points" :key="point.date">
                <th scope="row">{{ point.date }}</th>
                <td>
                  {{
                    point.value == null
                      ? '—'
                      : formatMetric(point.value, metric)
                  }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </details>
    </div>
  </AppSectionCard>
</template>

<style scoped>
.dashboard-trend {
  display: grid;
  gap: 10px;
  padding: 2px 20px 20px;
}

.dashboard-trend__summary {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.88rem;
}

.dashboard-trend__chart {
  width: 100%;
  height: 320px;
}

.dashboard-trend__data {
  border-top: 1px solid rgba(var(--app-shell-border), 0.45);
  padding-top: 10px;
}

.dashboard-trend__data summary {
  width: fit-content;
  cursor: pointer;
  color: rgb(var(--v-theme-primary));
  font-size: 0.86rem;
  font-weight: 700;
}

.dashboard-trend__data-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
}

.dashboard-trend__table-scroll {
  max-height: 280px;
  margin-top: 8px;
  overflow: auto;
}

.dashboard-trend table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.84rem;
}

.dashboard-trend caption {
  padding: 8px 0;
  text-align: left;
  font-weight: 700;
}

.dashboard-trend th,
.dashboard-trend td {
  padding: 8px 10px;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.36);
  text-align: left;
}

@media (max-width: 599px) {
  .dashboard-trend {
    padding: 0 12px 16px;
  }

  .dashboard-trend__chart {
    height: 260px;
  }

  .dashboard-trend :deep(.v-btn-toggle) {
    width: 100%;
  }
}
</style>
