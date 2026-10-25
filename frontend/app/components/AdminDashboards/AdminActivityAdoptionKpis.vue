<script setup lang="ts">
import type { DashboardAdoption } from '~/types/user-service'
import { formatNumber } from '~/utils/formatters'

const props = withDefaults(
  defineProps<{
    adoption?: DashboardAdoption | null
    loading?: boolean
  }>(),
  {
    adoption: null,
    loading: false,
  },
)

const cards = computed(() => [
  {
    caption: 'Used this period',
    icon: 'mdi-account-check-outline',
    label: 'Active users',
    tone: 'info',
    value:
      props.adoption == null ? '—' : formatNumber(props.adoption.activeUsers),
  },
  {
    caption: 'Used this period',
    icon: 'mdi-account-cog-outline',
    label: 'Active service accounts',
    tone: 'secondary',
    value:
      props.adoption == null
        ? '—'
        : formatNumber(props.adoption.activeServiceAccounts),
  },
  {
    caption: 'Used this period',
    icon: 'mdi-key-chain',
    label: 'Active virtual keys',
    tone: 'primary',
    value:
      props.adoption == null
        ? '—'
        : formatNumber(props.adoption.activeVirtualKeys),
  },
])
</script>

<template>
  <section
    class="admin-activity-adoption-kpis"
    aria-label="Adoption"
    :aria-busy="props.loading"
  >
    <article
      v-for="card in cards"
      :key="card.label"
      class="admin-activity-adoption-kpis__card"
    >
      <v-avatar :color="card.tone" size="40" variant="tonal">
        <v-icon :icon="card.icon" size="22" />
      </v-avatar>

      <div class="admin-activity-adoption-kpis__copy">
        <span>{{ card.label }}</span>
        <strong v-if="!(props.loading && props.adoption == null)">
          {{ card.value }}
        </strong>
        <span v-else class="admin-activity-adoption-kpis__skeleton" />
        <small>{{ card.caption }}</small>
      </div>
    </article>
  </section>
</template>

<style scoped>
.admin-activity-adoption-kpis {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.admin-activity-adoption-kpis__card {
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

.admin-activity-adoption-kpis__copy {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.admin-activity-adoption-kpis__copy > span:first-child {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.78rem;
  font-weight: 750;
  line-height: 1.3;
  text-transform: uppercase;
}

.admin-activity-adoption-kpis__copy strong {
  overflow-wrap: anywhere;
  color: rgb(var(--app-shell-text-primary));
  font-size: 1.45rem;
  line-height: 1.2;
}

.admin-activity-adoption-kpis__copy small {
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.35;
}

.admin-activity-adoption-kpis__skeleton {
  width: 70px;
  height: 25px;
  border-radius: 5px;
  background: rgba(var(--app-shell-border), 0.4);
  animation: admin-activity-adoption-pulse 1.2s ease-in-out infinite;
}

@keyframes admin-activity-adoption-pulse {
  50% {
    opacity: 0.48;
  }
}

@media (prefers-reduced-motion: reduce) {
  .admin-activity-adoption-kpis__skeleton {
    animation: none;
  }
}

@media (max-width: 959px) {
  .admin-activity-adoption-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 599px) {
  .admin-activity-adoption-kpis {
    grid-template-columns: 1fr;
  }
}
</style>
