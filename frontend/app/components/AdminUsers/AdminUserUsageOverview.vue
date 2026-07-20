<script setup lang="ts">
import type { DashboardOverviewResponse } from '~/types/user-service'
import {
  formatCurrencyUsd,
  formatDurationMs,
  formatNumber,
} from '~/utils/formatters'

const props = defineProps<{
  data: DashboardOverviewResponse | null
  error: string | null
  loading: boolean
}>()

const emit = defineEmits<{
  retry: []
}>()

const daily = computed(() => props.data?.daily ?? [])
const initialError = computed(() => Boolean(props.error && !props.data))
const initialLoading = computed(() => props.loading && !props.data)
const estimatedCost = computed(
  () => props.data?.totals.estimatedCost?.totalUsd ?? null,
)
const totalTokens = computed(() => props.data?.totals.totalTokens ?? null)
const requests = computed(() => props.data?.totals.requests ?? null)
const totalDurationMs = computed(() => props.data?.totalDurationMs ?? null)
const hasUsage = computed(() => {
  if (!props.data) {
    return false
  }

  return (
    props.data.totals.requests > 0 ||
    props.data.totals.totalTokens > 0 ||
    props.data.totalDurationMs > 0 ||
    props.data.daily.some(
      (item) => item.requests > 0 || item.totalTokens > 0,
    )
  )
})
const costCaption = computed(() =>
  props.data && !props.data.totals.estimatedCost
    ? 'Cost estimate unavailable'
    : 'User usage estimate',
)
</script>

<template>
  <div class="admin-user-usage-overview">
    <v-alert
      v-if="props.error"
      class="admin-user-usage-overview__alert"
      type="warning"
      variant="tonal"
      rounded="lg"
    >
      <div class="admin-user-usage-overview__alert-content">
        <span>{{ props.error }}</span>
        <v-btn
          prepend-icon="mdi-refresh"
          size="small"
          variant="text"
          @click="emit('retry')"
        >
          Retry
        </v-btn>
      </div>
    </v-alert>

    <template v-if="!initialError">
      <div class="admin-user-usage-overview__kpis">
        <DashboardKpiCard
          :animate="false"
          :caption="costCaption"
          color="info"
          :formatter="formatCurrencyUsd"
          icon="mdi-currency-usd"
          :loading="props.loading"
          title="Estimated cost"
          :value="estimatedCost"
        />
        <DashboardKpiCard
          caption="Tokens processed"
          color="warning"
          :formatter="formatNumber"
          icon="mdi-counter"
          :loading="props.loading"
          title="Total tokens"
          :value="totalTokens"
        />
        <DashboardKpiCard
          caption="Requests handled"
          color="primary"
          :formatter="formatNumber"
          icon="mdi-message-text-outline"
          :loading="props.loading"
          title="Requests"
          :value="requests"
        />
        <DashboardKpiCard
          caption="Completed requests"
          color="success"
          :formatter="formatDurationMs"
          icon="mdi-timer-outline"
          :loading="props.loading"
          title="Total duration"
          :value="totalDurationMs"
        />
      </div>

      <section class="admin-user-usage-overview__activity">
        <div class="admin-user-usage-overview__activity-header">
          <div>
            <h3>Daily activity</h3>
            <p>Requests and token usage across the selected window.</p>
          </div>
          <v-chip
            v-if="hasUsage && daily.length > 0"
            color="primary"
            label
            prepend-icon="mdi-pulse"
            size="small"
            variant="tonal"
          >
            {{ formatNumber(requests) }} requests
          </v-chip>
        </div>

        <div class="admin-user-usage-overview__activity-body">
          <div
            v-if="initialLoading"
            aria-busy="true"
            class="admin-user-usage-overview__skeleton"
          >
            <span />
            <span />
            <span />
          </div>
          <AppEmptyState
            v-else-if="!hasUsage"
            compact
            icon="mdi-chart-timeline-variant-shimmer"
            text="This user has no usage in the selected window."
            title="No usage yet"
          />
          <AppEmptyState
            v-else-if="daily.length === 0"
            compact
            icon="mdi-chart-timeline-variant-shimmer"
            text="Daily usage details are unavailable for this window."
            title="No daily activity"
          />
          <DashboardUsageLineChart
            v-else
            class="admin-user-usage-overview__plot"
            :daily="daily"
          />
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.admin-user-usage-overview {
  display: grid;
  gap: 18px;
}

.admin-user-usage-overview__alert-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.admin-user-usage-overview__kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.admin-user-usage-overview__activity {
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: 8px;
  background: rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
}

.admin-user-usage-overview__activity-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px 8px;
}

.admin-user-usage-overview__activity-header h3 {
  margin: 0;
  color: rgb(var(--app-shell-text-primary));
  font-size: 1rem;
}

.admin-user-usage-overview__activity-header p {
  margin: 3px 0 0;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.86rem;
}

.admin-user-usage-overview__activity-body,
.admin-user-usage-overview__plot,
.admin-user-usage-overview__skeleton {
  height: 286px;
}

.admin-user-usage-overview__activity-body {
  display: grid;
  align-items: center;
  padding: 4px 14px 14px;
}

.admin-user-usage-overview__skeleton {
  display: grid;
  align-content: end;
  gap: 18px;
  overflow: hidden;
}

.admin-user-usage-overview__skeleton span {
  display: block;
  height: 18px;
  border-radius: 999px;
  background: rgba(var(--app-shell-border), 0.42);
  animation: admin-user-usage-overview-pulse 1.2s ease-in-out infinite;
}

.admin-user-usage-overview__skeleton span:nth-child(1) {
  width: 92%;
}

.admin-user-usage-overview__skeleton span:nth-child(2) {
  width: 68%;
}

.admin-user-usage-overview__skeleton span:nth-child(3) {
  width: 78%;
}

@media (max-width: 900px) {
  .admin-user-usage-overview__kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .admin-user-usage-overview__kpis {
    grid-template-columns: minmax(0, 1fr);
  }

  .admin-user-usage-overview__activity-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .admin-user-usage-overview__activity-body,
  .admin-user-usage-overview__plot,
  .admin-user-usage-overview__skeleton {
    height: 250px;
  }
}

@keyframes admin-user-usage-overview-pulse {
  50% {
    opacity: 0.46;
  }
}
</style>
