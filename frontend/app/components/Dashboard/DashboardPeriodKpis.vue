<script setup lang="ts">
import type {
  DashboardCockpitOverviewResponse,
  DashboardPreviousPeriod,
} from '~/types/user-service'
import {
  formatCompactNumber,
  formatCurrencyUsd,
  formatDate,
  formatDurationMs,
  formatNumber,
} from '~/utils/formatters'

const props = defineProps<{
  overview: DashboardCockpitOverviewResponse
}>()

interface KpiView {
  caption: string
  delta: string | null
  icon: string
  label: string
  tone: string
  value: string
}

const previous = computed<DashboardPreviousPeriod | null>(
  () => props.overview.previous ?? null,
)
const costCoverage = computed(() =>
  props.overview.costCoverage ??
  (props.overview.totals.estimatedCost
    ? ({ status: 'complete' } as const)
    : ({ status: 'unavailable' } as const)),
)
const showDeltas = computed(
  () => props.overview.window !== 'all' && previous.value != null,
)

const primaryKpis = computed<KpiView[]>(() => [
  {
    caption: 'Completed API traffic',
    delta: deltaLabel(
      props.overview.totals.requests,
      previous.value?.totals.requests,
    ),
    icon: 'mdi-api',
    label: 'API requests',
    tone: 'primary',
    value: formatCompactNumber(props.overview.totals.requests),
  },
  {
    caption: `${formatNumber(props.overview.totals.totalTokens)} total tokens`,
    delta: deltaLabel(
      props.overview.totals.totalTokens,
      previous.value?.totals.totalTokens,
    ),
    icon: 'mdi-counter',
    label: 'Tokens',
    tone: 'warning',
    value: formatCompactNumber(props.overview.totals.totalTokens),
  },
  {
    caption: costCaption.value,
    delta: costDelta.value,
    icon: 'mdi-currency-usd',
    label: 'Estimated cost',
    tone: 'success',
    value: costValue.value,
  },
])

const secondaryKpis = computed<KpiView[]>(() => [
  {
    caption: 'Calls reported by model responses',
    delta: deltaLabel(
      props.overview.totals.toolCalls,
      previous.value?.totals.toolCalls,
    ),
    icon: 'mdi-tools',
    label: 'Tool calls',
    tone: 'info',
    value: formatCompactNumber(props.overview.totals.toolCalls),
  },
  {
    caption: 'Average across API requests',
    delta: tokensPerRequestDelta.value,
    icon: 'mdi-chart-donut',
    label: 'Tokens per request',
    tone: 'primary',
    value: tokensPerRequestLabel(
      props.overview.totals.totalTokens,
      props.overview.totals.requests,
    ),
  },
  {
    caption: 'Completed requests only',
    delta: durationDelta.value,
    icon: 'mdi-timer-outline',
    label: 'Average completed request duration',
    tone: 'secondary',
    value:
      props.overview.averageDurationMs == null
        ? '—'
        : formatDurationMs(props.overview.averageDurationMs),
  },
])

const costValue = computed(() => {
  if (
    costCoverage.value.status === 'unavailable' ||
    !props.overview.totals.estimatedCost
  ) {
    return '—'
  }
  return formatCurrencyUsd(props.overview.totals.estimatedCost.totalUsd)
})

const costCaption = computed(() => {
  const coverage = costCoverage.value
  if (coverage.status === 'unavailable') {
    return 'Current-price estimate unavailable'
  }
  if (coverage.status === 'partial') {
    return coverage.startsAt
      ? `Estimated since ${formatDate(coverage.startsAt)}`
      : 'Partial current-price estimate'
  }
  return 'Estimated at current prices'
})

const costDelta = computed(() => {
  if (
    costCoverage.value.status !== 'complete' ||
    previous.value?.costCoverage?.status !== 'complete'
  ) {
    return null
  }

  return deltaLabel(
    props.overview.totals.estimatedCost?.totalUsd,
    previous.value?.totals.estimatedCost?.totalUsd,
  )
})

const tokensPerRequestDelta = computed(() =>
  deltaLabel(
    ratio(
      props.overview.totals.totalTokens,
      props.overview.totals.requests,
    ),
    ratio(
      previous.value?.totals.totalTokens,
      previous.value?.totals.requests,
    ),
  ),
)

const durationDelta = computed(() =>
  deltaLabel(
    props.overview.averageDurationMs,
    previous.value?.averageDurationMs,
  ),
)

function ratio(
  numerator: number | null | undefined,
  denominator: number | null | undefined,
) {
  if (numerator == null || !denominator) return null
  return numerator / denominator
}

function tokensPerRequestLabel(tokens: number, requests: number) {
  const value = ratio(tokens, requests)
  if (value == null) return '—'
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 }).format(
    value,
  )
}

function deltaLabel(
  current: number | null | undefined,
  prior: number | null | undefined,
) {
  if (!showDeltas.value || current == null || prior == null) return null
  if (prior === 0) return current === 0 ? 'No change vs previous period' : 'New vs previous period'

  const percent = ((current - prior) / Math.abs(prior)) * 100
  const formatted = new Intl.NumberFormat('en-US', {
    maximumFractionDigits: 1,
    signDisplay: 'exceptZero',
  }).format(percent)
  return `${formatted}% vs previous period`
}
</script>

<template>
  <section class="dashboard-period-kpis" aria-labelledby="period-kpis-title">
    <div class="dashboard-period-kpis__heading">
      <div>
        <p class="dashboard-period-kpis__eyebrow">Selected period</p>
        <h2 id="period-kpis-title">What you consumed</h2>
      </div>
      <span v-if="showDeltas">Compared with the preceding period of equal length</span>
    </div>

    <div class="dashboard-period-kpis__grid dashboard-period-kpis__grid--primary">
      <article
        v-for="kpi in primaryKpis"
        :key="kpi.label"
        class="dashboard-period-kpis__card"
      >
        <v-avatar :color="kpi.tone" size="40" variant="tonal">
          <v-icon :icon="kpi.icon" size="22" />
        </v-avatar>
        <div class="dashboard-period-kpis__copy">
          <span>{{ kpi.label }}</span>
          <strong>{{ kpi.value }}</strong>
          <small>{{ kpi.caption }}</small>
          <small v-if="kpi.delta" class="dashboard-period-kpis__delta">
            {{ kpi.delta }}
          </small>
        </div>
      </article>
    </div>

    <div class="dashboard-period-kpis__grid dashboard-period-kpis__grid--secondary">
      <article
        v-for="kpi in secondaryKpis"
        :key="kpi.label"
        class="dashboard-period-kpis__card dashboard-period-kpis__card--secondary"
      >
        <v-avatar :color="kpi.tone" size="36" variant="tonal">
          <v-icon :icon="kpi.icon" size="20" />
        </v-avatar>
        <div class="dashboard-period-kpis__copy">
          <div class="dashboard-period-kpis__label-row">
            <span>{{ kpi.label }}</span>
            <v-tooltip
              v-if="kpi.label === 'Average completed request duration'"
              text="Elapsed time for the complete response, including streamed output. In-flight requests are excluded."
              location="top"
            >
              <template #activator="{ props: tooltipProps }">
                <v-btn
                  v-bind="tooltipProps"
                  aria-label="About average completed request duration"
                  class="dashboard-period-kpis__help"
                  density="compact"
                  icon="mdi-information-outline"
                  size="x-small"
                  variant="text"
                />
              </template>
            </v-tooltip>
          </div>
          <strong>{{ kpi.value }}</strong>
          <small>{{ kpi.caption }}</small>
          <small v-if="kpi.delta" class="dashboard-period-kpis__delta">
            {{ kpi.delta }}
          </small>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.dashboard-period-kpis {
  display: grid;
  gap: 14px;
}

.dashboard-period-kpis__heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
}

.dashboard-period-kpis__heading h2,
.dashboard-period-kpis__eyebrow {
  margin: 0;
}

.dashboard-period-kpis__heading h2 {
  font-size: 1.15rem;
}

.dashboard-period-kpis__heading > span,
.dashboard-period-kpis__eyebrow {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.8rem;
}

.dashboard-period-kpis__eyebrow {
  margin-bottom: 3px;
  font-weight: 750;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.dashboard-period-kpis__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.dashboard-period-kpis__card {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
  padding: 17px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: var(--app-card-radius);
  background: rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
}

.dashboard-period-kpis__card--secondary {
  background: rgba(var(--app-shell-surface-muted), 0.42);
}

.dashboard-period-kpis__copy {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.dashboard-period-kpis__copy > span,
.dashboard-period-kpis__label-row > span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.78rem;
  font-weight: 750;
  line-height: 1.3;
  text-transform: uppercase;
}

.dashboard-period-kpis__copy strong {
  overflow-wrap: anywhere;
  font-size: 1.45rem;
  line-height: 1.2;
}

.dashboard-period-kpis__copy small {
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.35;
}

.dashboard-period-kpis__delta {
  margin-top: 3px;
  color: rgb(var(--app-shell-text-primary)) !important;
  font-weight: 650;
}

.dashboard-period-kpis__label-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.dashboard-period-kpis__help {
  width: 28px;
  height: 28px;
  min-width: 28px !important;
  min-height: 28px !important;
}

@media (max-width: 959px) {
  .dashboard-period-kpis__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 599px) {
  .dashboard-period-kpis__heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .dashboard-period-kpis__grid {
    grid-template-columns: 1fr;
  }
}
</style>
