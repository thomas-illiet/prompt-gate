<script setup lang="ts">
import type { UsageWindow } from '~/types/user-service'
import type { AppSignalVisualName } from '~/types/theme'
import { formatDateTime } from '~/utils/formatters'

const props = withDefaults(
  defineProps<{
    copy: string
    controlsLabel?: string
    icon?: string
    kicker?: string
    loading: boolean
    periodLabel: string
    refreshLabel?: string
    title: string
    updatedAt?: string | null
    visual?: AppSignalVisualName
  }>(),
  {
    controlsLabel: 'Dashboard controls',
    icon: 'mdi-monitor-dashboard',
    kicker: 'Usage cockpit',
    refreshLabel: 'Refresh all dashboard data',
    updatedAt: null,
    visual: 'radar',
  },
)

const emit = defineEmits<{
  refresh: []
}>()

const window = defineModel<UsageWindow>('window', { required: true })
</script>

<template>
  <AppPageHeader
    :copy="props.copy"
    density="compact"
    :icon="props.icon"
    :kicker="props.kicker"
    :title="props.title"
    :visual="props.visual"
  >
    <template #meta>
      <div class="dashboard-page-header__period" aria-live="polite">
        <v-icon icon="mdi-calendar-range" size="17" />
        <span>{{ props.periodLabel }}</span>
        <span v-if="props.updatedAt" aria-hidden="true">·</span>
        <span v-if="props.updatedAt">
          Updated {{ formatDateTime(props.updatedAt) }}
        </span>
      </div>
    </template>

    <template #footer>
      <div
        class="dashboard-page-header__controls"
        :aria-label="props.controlsLabel"
      >
        <DashboardTimeRangeSelect v-model="window" />
        <v-btn
          class="dashboard-page-header__refresh"
          color="primary"
          prepend-icon="mdi-refresh"
          variant="outlined"
          rounded="lg"
          :loading="props.loading"
          :aria-label="props.refreshLabel"
          @click="emit('refresh')"
        >
          Refresh
        </v-btn>
      </div>
    </template>
  </AppPageHeader>
</template>

<style scoped>
.dashboard-page-header__period {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
  font-weight: 650;
}

.dashboard-page-header__controls {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  padding: 12px 18px;
}

.dashboard-page-header__refresh {
  min-height: 46px;
  font-weight: 700;
  text-transform: none;
}

@container (max-width: 560px) {
  .dashboard-page-header__controls {
    align-items: stretch;
    flex-direction: column;
    padding: 12px 16px 14px;
  }

  .dashboard-page-header__refresh {
    width: 100%;
  }
}
</style>
