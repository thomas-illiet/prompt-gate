<script setup lang="ts">
import type {
  AdminQuotaResetEventsSummary,
  AdminQuotaStatesSummary,
} from '~/types/quota-resets'
import { formatNumber } from '~/utils/formatters'

const props = defineProps<{
  eventsAvailable: boolean
  eventsLoading: boolean
  eventsSummary: AdminQuotaResetEventsSummary
  statesAvailable: boolean
  statesLoading: boolean
  statesSummary: AdminQuotaStatesSummary
}>()

const cards = computed(() => [
  {
    caption: props.statesAvailable
      ? 'SQL projections currently tracked'
      : 'Projection summary unavailable',
    icon: 'mdi-database-clock-outline',
    label: 'Active projections',
    loading: props.statesLoading,
    tone: 'primary',
    value: props.statesAvailable
      ? formatNumber(props.statesSummary.projectedAccounts)
      : '—',
  },
  {
    caption: props.statesAvailable
      ? 'At least one window at 80% or more'
      : 'Projection summary unavailable',
    icon: 'mdi-alert-outline',
    label: 'Accounts at risk',
    loading: props.statesLoading,
    tone: 'warning',
    value: props.statesAvailable
      ? formatNumber(props.statesSummary.atRiskAccounts)
      : '—',
  },
  {
    caption: props.statesAvailable
      ? 'At least one exhausted window'
      : 'Projection summary unavailable',
    icon: 'mdi-alert-octagon-outline',
    label: 'Accounts exhausted',
    loading: props.statesLoading,
    tone: 'error',
    value: props.statesAvailable
      ? formatNumber(props.statesSummary.exhaustedAccounts)
      : '—',
  },
  {
    caption: props.eventsAvailable
      ? 'Monthly / administrator grants in period'
      : 'Event summary unavailable',
    icon: 'mdi-ticket-confirmation-outline',
    label: 'Credits granted',
    loading: props.eventsLoading,
    tone: 'info',
    value: props.eventsAvailable
      ? `${formatNumber(props.eventsSummary.monthlyGrants)} / ${formatNumber(
          props.eventsSummary.adminGrants,
        )}`
      : '—',
  },
  {
    caption: props.eventsAvailable
      ? 'Successfully applied in selected period'
      : 'Event summary unavailable',
    icon: 'mdi-check-decagram-outline',
    label: 'Resets applied',
    loading: props.eventsLoading,
    tone: 'success',
    value: props.eventsAvailable
      ? formatNumber(props.eventsSummary.appliedResetUses)
      : '—',
  },
  {
    caption: props.eventsAvailable
      ? 'Global backlog, independent of period'
      : 'Event summary unavailable',
    icon: 'mdi-progress-clock',
    label: 'Pending backlog',
    loading: props.eventsLoading,
    tone: 'secondary',
    value: props.eventsAvailable
      ? formatNumber(props.eventsSummary.pendingResetUsesTotal)
      : '—',
  },
])
</script>

<template>
  <section class="admin-quota-reset-kpis" aria-label="Quota reset summary">
    <article
      v-for="card in cards"
      :key="card.label"
      class="admin-quota-reset-kpis__card"
      :aria-busy="card.loading"
    >
      <v-avatar :color="card.tone" size="38" variant="tonal">
        <v-icon :icon="card.icon" size="21" />
      </v-avatar>
      <div>
        <span>{{ card.label }}</span>
        <strong v-if="!card.loading">{{ card.value }}</strong>
        <span v-else class="admin-quota-reset-kpis__skeleton" />
        <small>{{ card.caption }}</small>
      </div>
    </article>
  </section>
</template>

<style scoped>
.admin-quota-reset-kpis {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.admin-quota-reset-kpis__card {
  display: flex;
  gap: 13px;
  min-width: 0;
  padding: 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: var(--app-card-radius);
  background: rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
}

.admin-quota-reset-kpis__card > div {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.admin-quota-reset-kpis__card span:first-child {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.75rem;
  font-weight: 750;
  text-transform: uppercase;
}

.admin-quota-reset-kpis__card strong {
  overflow-wrap: anywhere;
  font-size: 1.35rem;
}

.admin-quota-reset-kpis__card small {
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.35;
}

.admin-quota-reset-kpis__skeleton {
  width: 70px;
  height: 25px;
  border-radius: 5px;
  background: rgba(var(--app-shell-border), 0.4);
}

@media (max-width: 959px) {
  .admin-quota-reset-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 599px) {
  .admin-quota-reset-kpis {
    grid-template-columns: 1fr;
  }
}
</style>
