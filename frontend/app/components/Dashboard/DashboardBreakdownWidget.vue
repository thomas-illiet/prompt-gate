<script setup lang="ts">
import type {
  DashboardBreakdownResponse,
  UsageWindow,
} from '~/types/user-service'
import { formatNumber } from '~/utils/formatters'

const props = withDefaults(
  defineProps<{
    chartType?: 'bar' | 'pie'
    emptyLabel: string
    endpoint: string
    icon: string
    showSetupAction?: boolean
    subtitle: string
    title: string
    window: UsageWindow
  }>(),
  {
    chartType: 'bar',
    showSetupAction: true,
  },
)

const widget = useDashboardWidget<DashboardBreakdownResponse>(
  () => props.endpoint,
  () => props.window,
)

const items = computed(() => widget.data.value?.items ?? [])
const showSkeleton = computed(() => widget.loading.value && !widget.data.value)
const showError = computed(() => widget.error.value && !widget.data.value)
const leader = computed(() => items.value[0] ?? null)
</script>

<template>
  <AppSectionCard
    :icon="props.icon"
    :title="props.title"
    :subtitle="props.subtitle"
  >
    <template v-if="leader" #actions>
      <v-chip
        size="small"
        label
        variant="tonal"
        color="primary"
        prepend-icon="mdi-trophy-outline"
      >
        {{ leader.name }} / {{ formatNumber(leader.totalTokens) }}
      </v-chip>
    </template>

    <div class="dashboard-breakdown-widget">
      <div
        v-if="showSkeleton"
        class="dashboard-breakdown-widget__skeleton"
        aria-busy="true"
      >
        <span v-for="index in 5" :key="index" />
      </div>

      <v-alert
        v-else-if="showError"
        type="warning"
        variant="tonal"
        class="dashboard-breakdown-widget__alert"
      >
        <div class="dashboard-breakdown-widget__alert-content">
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
        v-else-if="items.length === 0"
        class="dashboard-breakdown-widget__empty"
      >
        <AppEmptyState
          compact
          :icon="props.icon"
          :title="props.emptyLabel"
          text="Traffic will turn this panel into a ranked view."
        >
          <template v-if="props.showSetupAction" #actions>
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

      <DashboardBreakdownBarChart
        v-else-if="props.chartType !== 'pie'"
        class="dashboard-breakdown-widget__chart"
        :items="items"
      />

      <DashboardBreakdownPieChart
        v-else
        class="dashboard-breakdown-widget__chart"
        :items="items"
      />
    </div>
  </AppSectionCard>
</template>

<style scoped>
.dashboard-breakdown-widget {
  min-height: 300px;
  padding: 8px 16px 20px;
}

.dashboard-breakdown-widget__chart,
.dashboard-breakdown-widget__skeleton,
.dashboard-breakdown-widget__empty,
.dashboard-breakdown-widget__alert {
  min-height: 272px;
}

.dashboard-breakdown-widget__skeleton {
  display: grid;
  align-content: center;
  gap: 16px;
}

.dashboard-breakdown-widget__skeleton span {
  display: block;
  height: 22px;
  border-radius: 6px;
  background: rgba(var(--app-shell-border), 0.42);
  animation: dashboard-breakdown-pulse 1.2s ease-in-out infinite;
}

.dashboard-breakdown-widget__skeleton span:nth-child(1) {
  width: 92%;
}

.dashboard-breakdown-widget__skeleton span:nth-child(2) {
  width: 78%;
}

.dashboard-breakdown-widget__skeleton span:nth-child(3) {
  width: 64%;
}

.dashboard-breakdown-widget__skeleton span:nth-child(4) {
  width: 48%;
}

.dashboard-breakdown-widget__skeleton span:nth-child(5) {
  width: 34%;
}

.dashboard-breakdown-widget__empty {
  display: grid;
  align-items: center;
}

.dashboard-breakdown-widget__alert {
  display: grid;
  align-items: center;
}

.dashboard-breakdown-widget__alert-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

@keyframes dashboard-breakdown-pulse {
  50% {
    opacity: 0.46;
  }
}
</style>
