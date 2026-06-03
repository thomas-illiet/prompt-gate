<script setup lang="ts">
import type { ProfileTokenUsageSummary } from '~/composables/useProfileTokenUsage'
import {
  formatCompactNumber,
  formatDate,
  formatNumber,
} from '~/utils/formatters'

const props = defineProps<{
  error: string | null
  loading: boolean
  summary: ProfileTokenUsageSummary
}>()

const emit = defineEmits<{
  retry: []
}>()

const hasActivity = computed(() => props.summary.totalTokens > 0)
const activeDaysLabel = computed(() =>
  formatDayCount(props.summary.activeDays, 'active day', 'active days'),
)
const kpis = computed(() => [
  {
    caption: `${formatNumber(props.summary.totalTokens)} tokens total`,
    color: 'warning',
    icon: 'mdi-counter',
    label: 'Tokens 12 mois',
    value: formatCompactNumber(props.summary.totalTokens),
  },
  {
    caption: props.summary.peakDay
      ? formatDate(props.summary.peakDay.date)
      : 'No activity yet',
    color: 'primary',
    icon: 'mdi-trending-up',
    label: 'Pic journalier',
    value: formatCompactNumber(props.summary.peakDay?.totalTokens ?? 0),
  },
  {
    caption: `Longest ${formatDayCount(
      props.summary.longestStreakDays,
      'day',
      'days',
    )}`,
    color: 'success',
    icon: 'mdi-calendar-check-outline',
    label: 'Streak',
    value: formatDayCount(props.summary.currentStreakDays, 'day', 'days'),
  },
])

function formatDayCount(value: number, singular: string, plural: string) {
  return `${formatNumber(value)} ${value === 1 ? singular : plural}`
}
</script>

<template>
  <section class="profile-token-usage-section" data-test="profile-token-usage">
    <div class="profile-token-usage-section__kpis">
      <article
        v-for="kpi in kpis"
        :key="kpi.label"
        class="profile-token-usage-section__kpi"
        data-test="profile-token-kpi"
      >
        <v-avatar :color="kpi.color" variant="tonal" size="38">
          <v-icon :icon="kpi.icon" size="21" />
        </v-avatar>

        <div class="profile-token-usage-section__kpi-copy">
          <span class="profile-token-usage-section__kpi-label">
            {{ kpi.label }}
          </span>
          <strong class="profile-token-usage-section__kpi-value">
            {{ kpi.value }}
          </strong>
          <span class="profile-token-usage-section__kpi-caption">
            {{ kpi.caption }}
          </span>
        </div>
      </article>
    </div>

    <ProfileInfoCard
      icon="mdi-chart-box-outline"
      title="Token activity"
      subtitle="Daily token consumption over the last 12 months"
    >
      <template #actions>
        <v-chip label variant="tonal" color="primary" size="small">
          Daily
        </v-chip>
        <v-chip
          v-if="hasActivity"
          label
          variant="tonal"
          color="success"
          size="small"
        >
          {{ activeDaysLabel }}
        </v-chip>
      </template>

      <div class="profile-token-usage-section__body">
        <div
          v-if="props.loading"
          class="profile-token-usage-section__skeleton"
          aria-busy="true"
          data-test="profile-token-loading"
        >
          <span v-for="index in 91" :key="index" />
        </div>

        <v-alert
          v-else-if="props.error"
          type="warning"
          variant="tonal"
          rounded="lg"
          data-test="profile-token-error"
        >
          <div class="profile-token-usage-section__alert-content">
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

        <AppEmptyState
          v-else-if="!hasActivity"
          compact
          icon="mdi-chart-timeline-variant-shimmer"
          title="No token activity"
          text="Send a first request and the activity calendar will start filling in."
          data-test="profile-token-empty"
        />

        <ProfileTokenHeatmap
          v-else
          :days="props.summary.days"
          data-test="profile-token-heatmap"
        />
      </div>
    </ProfileInfoCard>
  </section>
</template>

<style scoped>
.profile-token-usage-section {
  display: grid;
  gap: 16px;
}

.profile-token-usage-section__kpis {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.profile-token-usage-section__kpi {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: var(--app-card-radius);
  background:
    linear-gradient(
      180deg,
      rgba(var(--app-shell-surface-strong), 0.96),
      rgba(var(--app-shell-surface), 0.96)
    ),
    rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
}

.profile-token-usage-section__kpi-copy {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.profile-token-usage-section__kpi-label {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.78rem;
  font-weight: 750;
  text-transform: uppercase;
}

.profile-token-usage-section__kpi-value {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 1.35rem;
  font-weight: 800;
  line-height: 1.2;
}

.profile-token-usage-section__kpi-caption {
  min-width: 0;
  overflow-wrap: anywhere;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
  line-height: 1.35;
}

.profile-token-usage-section__body {
  min-height: 178px;
  padding: 2px 24px 24px;
}

.profile-token-usage-section__skeleton {
  display: grid;
  grid-template-columns: repeat(13, 12px);
  gap: 4px;
  align-content: start;
  min-height: 132px;
  padding-top: 12px;
}

.profile-token-usage-section__skeleton span {
  width: 12px;
  height: 12px;
  border-radius: 3px;
  background: rgba(var(--app-shell-border), 0.38);
  animation: profile-token-skeleton-pulse 1.2s ease-in-out infinite;
}

.profile-token-usage-section__alert-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

@keyframes profile-token-skeleton-pulse {
  50% {
    opacity: 0.48;
  }
}

@media (max-width: 960px) {
  .profile-token-usage-section__kpis {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .profile-token-usage-section__body {
    padding: 0 16px 20px;
  }

  .profile-token-usage-section__alert-content {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
