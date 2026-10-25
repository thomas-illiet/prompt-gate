<script setup lang="ts">
import { downloadDailyUsageCsv } from '~/utils/daily-usage-csv'
import {
  formatCompactNumber,
  formatDate,
  formatNumber,
} from '~/utils/formatters'

const props = defineProps<{
  refreshVersion: number
}>()

const annual = useProfileTokenUsage()

watch(
  () => props.refreshVersion,
  () => {
    void annual.reload()
  },
)

const peakCaption = computed(() =>
  annual.summary.value.peakDay
    ? formatDate(annual.summary.value.peakDay.date)
    : 'No activity',
)

function exportDailyData() {
  const days = annual.summary.value.days
  const firstDate = days[0]?.date
  const lastDate = days[days.length - 1]?.date
  if (!firstDate || !lastDate) return

  downloadDailyUsageCsv(
    days,
    'annual',
    `promptgate-annual-activity-${firstDate}-to-${lastDate}.csv`,
  )
}
</script>

<template>
  <AppSectionCard
    icon="mdi-calendar-blank-multiple"
    title="Annual activity"
    subtitle="Your token activity across the last 365 days"
  >
    <div class="dashboard-annual-activity">
      <div
        v-if="annual.loading.value && !annual.activity.value"
        class="dashboard-annual-activity__loading"
        aria-label="Loading annual activity"
      >
        <span v-for="index in 84" :key="index" />
      </div>

      <v-alert
        v-else-if="annual.error.value"
        type="warning"
        variant="tonal"
        rounded="lg"
      >
        <div class="dashboard-annual-activity__alert">
          <span>{{ annual.error.value }}</span>
          <v-btn
            size="small"
            variant="text"
            prepend-icon="mdi-refresh"
            @click="annual.reload"
          >
            Retry
          </v-btn>
        </div>
      </v-alert>

      <template v-else>
        <div class="dashboard-annual-activity__kpis">
          <article>
            <span>Total tokens</span>
            <strong>{{
              formatCompactNumber(annual.summary.value.totalTokens)
            }}</strong>
            <small
              >{{ formatNumber(annual.summary.value.totalTokens) }} over 365
              days</small
            >
          </article>
          <article>
            <span>Active days</span>
            <strong>{{ formatNumber(annual.summary.value.activeDays) }}</strong>
            <small>Days with recorded token usage</small>
          </article>
          <article>
            <span>Daily peak</span>
            <strong>{{
              formatCompactNumber(annual.summary.value.maxTokens)
            }}</strong>
            <small>{{ peakCaption }}</small>
          </article>
        </div>

        <p
          v-if="annual.summary.value.totalTokens === 0"
          class="dashboard-annual-activity__empty"
        >
          No token activity was recorded during the last 365 days.
        </p>

        <template v-else>
          <p class="dashboard-annual-activity__summary">
            {{ formatNumber(annual.summary.value.activeDays) }} active days from
            {{ formatDate(annual.summary.value.startsAt) }} to
            {{ formatDate(annual.summary.value.endsAt) }}.
          </p>
          <ProfileTokenHeatmap :days="annual.summary.value.days" />

          <details class="dashboard-annual-activity__data">
            <summary>View daily data</summary>
            <div class="dashboard-annual-activity__data-actions">
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
            <div class="dashboard-annual-activity__table-scroll">
              <table>
                <caption>
                  Daily annual activity
                </caption>
                <thead>
                  <tr>
                    <th scope="col">Date</th>
                    <th scope="col">API requests</th>
                    <th scope="col">Tokens</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="day in annual.summary.value.days" :key="day.date">
                    <th scope="row">{{ day.date }}</th>
                    <td>{{ formatNumber(day.requests) }}</td>
                    <td>{{ formatNumber(day.totalTokens) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </details>
        </template>
      </template>
    </div>
  </AppSectionCard>
</template>

<style scoped>
.dashboard-annual-activity {
  display: grid;
  gap: 18px;
  padding: 20px 24px 24px;
}

.dashboard-annual-activity__kpis {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.dashboard-annual-activity__kpis article {
  display: grid;
  gap: 3px;
  padding: 14px;
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.45);
}

.dashboard-annual-activity__kpis span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.77rem;
  font-weight: 750;
  text-transform: uppercase;
}

.dashboard-annual-activity__kpis strong {
  font-size: 1.35rem;
}

.dashboard-annual-activity__kpis small,
.dashboard-annual-activity__summary,
.dashboard-annual-activity__empty {
  color: rgb(var(--app-shell-text-secondary));
}

.dashboard-annual-activity__summary,
.dashboard-annual-activity__empty {
  margin: 0;
  font-size: 0.88rem;
}

.dashboard-annual-activity__empty {
  padding: 24px;
  border: 1px dashed rgba(var(--app-shell-border), 0.55);
  border-radius: var(--app-card-radius);
  text-align: center;
}

.dashboard-annual-activity__loading {
  display: grid;
  grid-template-columns: repeat(12, minmax(8px, 1fr));
  gap: 5px;
  min-height: 140px;
}

.dashboard-annual-activity__loading span {
  aspect-ratio: 1;
  border-radius: 3px;
  background: rgba(var(--app-shell-border), 0.34);
}

.dashboard-annual-activity__alert {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dashboard-annual-activity__data {
  border-top: 1px solid rgba(var(--app-shell-border), 0.45);
  padding-top: 10px;
}

.dashboard-annual-activity__data summary {
  width: fit-content;
  cursor: pointer;
  color: rgb(var(--v-theme-primary));
  font-size: 0.86rem;
  font-weight: 700;
}

.dashboard-annual-activity__data-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
}

.dashboard-annual-activity__table-scroll {
  max-height: 300px;
  margin-top: 8px;
  overflow: auto;
}

.dashboard-annual-activity table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.84rem;
}

.dashboard-annual-activity caption {
  padding: 8px 0;
  text-align: left;
  font-weight: 700;
}

.dashboard-annual-activity th,
.dashboard-annual-activity td {
  padding: 8px 10px;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.36);
  text-align: left;
}

@media (min-width: 600px) {
  .dashboard-annual-activity__kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 960px) {
  .dashboard-annual-activity__kpis {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 599px) {
  .dashboard-annual-activity {
    padding: 16px 16px 20px;
  }
}
</style>
