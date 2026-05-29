<script setup lang="ts">
import type {
  DashboardActivityResponse,
  DashboardScope,
  UsageWindow,
} from '~/types/user-service'
import { formatNumber } from '~/utils/formatters'

const props = defineProps<{
  scope: DashboardScope
  window: UsageWindow
}>()

const endpoint = computed(() =>
  props.scope === 'global'
    ? '/api/v1/admin/dashboard/activity'
    : '/api/v1/me/dashboard/activity',
)
const widget = useDashboardWidget<DashboardActivityResponse>(
  endpoint,
  () => props.window,
)

const daily = computed(() => widget.data.value?.daily ?? [])
const showSkeleton = computed(() => widget.loading.value && !widget.data.value)
const showError = computed(() => widget.error.value && !widget.data.value)
const activitySummary = computed(() => {
  const requests = daily.value.reduce((total, item) => total + item.requests, 0)
  const completionInputTokens = daily.value.reduce(
    (total, item) => total + item.completionInputTokens,
    0,
  )
  const completionOutputTokens = daily.value.reduce(
    (total, item) => total + item.completionOutputTokens,
    0,
  )
  const embeddingTokens = daily.value.reduce(
    (total, item) => total + item.embeddingTokens,
    0,
  )

  return {
    completionInputTokens,
    completionOutputTokens,
    embeddingTokens,
    requests,
  }
})
</script>

<template>
  <AppSectionCard
    icon="mdi-chart-line"
    title="Daily activity"
    subtitle="Usage trend"
  >
    <template v-if="daily.length > 0" #actions>
      <v-chip
        size="small"
        label
        variant="tonal"
        color="primary"
        prepend-icon="mdi-pulse"
      >
        {{ formatNumber(activitySummary.requests) }} requests
      </v-chip>
      <v-chip
        size="small"
        label
        variant="tonal"
        color="success"
        prepend-icon="mdi-import"
      >
        {{ formatNumber(activitySummary.completionInputTokens) }} input
      </v-chip>
      <v-chip
        size="small"
        label
        variant="tonal"
        color="warning"
        prepend-icon="mdi-export"
      >
        {{ formatNumber(activitySummary.completionOutputTokens) }} output
      </v-chip>
      <v-chip
        size="small"
        label
        variant="tonal"
        color="info"
        prepend-icon="mdi-vector-point"
      >
        {{ formatNumber(activitySummary.embeddingTokens) }} embedding
      </v-chip>
    </template>

    <div class="dashboard-activity-chart">
      <div
        v-if="showSkeleton"
        class="dashboard-activity-chart__skeleton"
        aria-busy="true"
      >
        <span />
        <span />
        <span />
      </div>

      <v-alert
        v-else-if="showError"
        type="warning"
        variant="tonal"
        class="dashboard-activity-chart__alert"
      >
        <div class="dashboard-activity-chart__alert-content">
          <span>{{ widget.error.value }}</span>
          <v-btn
            prepend-icon="mdi-refresh"
            size="small"
            variant="text"
            @click="widget.reload"
          >
            Retry
          </v-btn>
        </div>
      </v-alert>

      <div
        v-else-if="daily.length === 0"
        class="dashboard-activity-chart__empty"
      >
        <AppEmptyState
          icon="mdi-chart-timeline-variant-shimmer"
          title="No activity yet"
          text="Send a first request and this timeline will start drawing signal."
        >
          <template #actions>
            <v-btn
              to="/help"
              color="primary"
              variant="tonal"
              rounded="lg"
              prepend-icon="mdi-help-circle-outline"
            >
              Open setup guide
            </v-btn>
          </template>
        </AppEmptyState>
      </div>

      <DashboardUsageLineChart
        v-else
        class="dashboard-activity-chart__plot"
        :daily="daily"
      />
    </div>
  </AppSectionCard>
</template>

<style scoped>
.dashboard-activity-chart {
  min-height: 340px;
  padding: 12px 16px 20px;
}

.dashboard-activity-chart__plot,
.dashboard-activity-chart__skeleton,
.dashboard-activity-chart__alert,
.dashboard-activity-chart__empty {
  height: 308px;
}

.dashboard-activity-chart__skeleton {
  position: relative;
  display: grid;
  align-content: end;
  gap: 18px;
  overflow: hidden;
}

.dashboard-activity-chart__skeleton::before {
  position: absolute;
  inset: 24px 0 0;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.44);
  border-left: 1px solid rgba(var(--app-shell-border), 0.44);
  content: '';
}

.dashboard-activity-chart__skeleton span {
  display: block;
  height: 18px;
  border-radius: 999px;
  background: rgba(var(--app-shell-border), 0.42);
  animation: dashboard-activity-pulse 1.2s ease-in-out infinite;
}

.dashboard-activity-chart__skeleton span:nth-child(1) {
  width: 92%;
}

.dashboard-activity-chart__skeleton span:nth-child(2) {
  width: 68%;
}

.dashboard-activity-chart__skeleton span:nth-child(3) {
  width: 78%;
}

.dashboard-activity-chart__alert {
  display: grid;
  align-items: center;
}

.dashboard-activity-chart__alert-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dashboard-activity-chart__empty {
  display: grid;
  align-items: center;
}

@keyframes dashboard-activity-pulse {
  50% {
    opacity: 0.46;
  }
}
</style>
