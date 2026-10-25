<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type {
  AdminQuotaResetActor,
  AdminQuotaResetEventItem,
  AdminQuotaResetEventStatusFilter,
  AdminQuotaResetEventTypeFilter,
  AdminQuotaResetEventWindow,
} from '~/types/quota-resets'
import { formatDateTime, formatNumber } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  error: string | null
  items: AdminQuotaResetEventItem[]
  loading: boolean
  page: number
  pageSize: number
  search: string
  sortBy: string
  sortDir: 'asc' | 'desc'
  status: AdminQuotaResetEventStatusFilter
  total: number
  type: AdminQuotaResetEventTypeFilter
  window: AdminQuotaResetEventWindow
}>()

const emit = defineEmits<{
  refresh: []
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:search': [value: string]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
  'update:status': [value: AdminQuotaResetEventStatusFilter]
  'update:type': [value: AdminQuotaResetEventTypeFilter]
  'update:window': [value: AdminQuotaResetEventWindow]
}>()

const headers: DataTableHeader[] = [
  { title: 'Target', key: 'user' },
  appTableCenteredColumn({ title: 'Event', key: 'type' }),
  appTableCenteredColumn({ title: 'Status', key: 'status' }),
  appTableCenteredColumn({
    title: 'Credit balance',
    key: 'balance',
    sortable: false,
  }),
  { title: 'Actor', key: 'actor', sortable: false },
  appTableCenteredColumn({
    title: 'Distribution / cutoff',
    key: 'context',
    sortable: false,
  }),
  appTableCenteredColumn({ title: 'Created', key: 'createdAt' }),
  appTableCenteredColumn({ title: 'Applied', key: 'appliedAt' }),
  appTableCenteredColumn({
    title: 'Reconciliation',
    key: 'reconciliation',
    sortable: false,
  }),
]

const typeOptions = [
  { title: 'All event types', value: 'all' },
  { title: 'Monthly grant', value: 'monthly_grant' },
  { title: 'Administrator grant', value: 'admin_grant' },
  { title: 'Quota reset used', value: 'quota_reset_used' },
]
const statusOptions = [
  { title: 'All statuses', value: 'all' },
  { title: 'Applied', value: 'applied' },
  { title: 'Pending', value: 'pending' },
]

function targetLink(item: AdminQuotaResetEventItem) {
  return {
    path:
      item.target.accountType === 'service'
        ? '/admin/service-accounts'
        : '/admin/users',
    query: { search: item.target.accountId },
  }
}

function typeLabel(type: AdminQuotaResetEventItem['type']) {
  if (type === 'monthly_grant') return 'Monthly grant'
  if (type === 'admin_grant') return 'Administrator grant'
  return 'Quota reset used'
}

function actorLabel(actor: AdminQuotaResetActor) {
  if (actor.displayName) return actor.displayName
  if (actor.type === 'system') return 'System'
  if (actor.type === 'admin_api_key') return 'Administrator API key'
  if (actor.type === 'admin_session') return 'Deleted administrator'
  return 'Deleted user'
}

function actorCaption(actor: AdminQuotaResetActor) {
  if (actor.type === 'admin_api_key') return 'Administrator API key'
  if (actor.type === 'admin_session') return 'Administrator session'
  if (actor.type === 'user') return 'User'
  return 'Scheduler'
}

function monthLabel(value: string | null) {
  if (!value) return '—'
  const date = new Date(`${value}-01T00:00:00Z`)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    timeZone: 'UTC',
    year: 'numeric',
  }).format(date)
}

function signedDelta(value: number) {
  return new Intl.NumberFormat('en-US', { signDisplay: 'always' }).format(value)
}
</script>

<template>
  <section class="admin-quota-events-section">
    <AppFilterCard
      icon="mdi-history"
      title="Credit and reset register"
      subtitle="Review grants and reset applications. The pending counter above remains global across every period."
    >
      <div class="admin-quota-events-section__filters">
        <v-btn-toggle
          :model-value="props.window"
          aria-label="Event period"
          color="primary"
          density="compact"
          mandatory
          variant="outlined"
          @update:model-value="
            emit('update:window', $event as AdminQuotaResetEventWindow)
          "
        >
          <v-btn value="7d">7 days</v-btn>
          <v-btn value="30d">30 days</v-btn>
          <v-btn value="all">All</v-btn>
        </v-btn-toggle>
        <v-text-field
          :model-value="props.search"
          clearable
          density="compact"
          hide-details
          label="Search targets"
          placeholder="Name or identifier"
          prepend-inner-icon="mdi-magnify"
          variant="outlined"
          @update:model-value="
            emit('update:search', ($event as string | null) ?? '')
          "
        />
        <v-select
          :items="typeOptions"
          :model-value="props.type"
          density="compact"
          hide-details
          label="Event type"
          prepend-inner-icon="mdi-swap-vertical"
          variant="outlined"
          @update:model-value="
            emit(
              'update:type',
              ($event as AdminQuotaResetEventTypeFilter | null) ?? 'all',
            )
          "
        />
        <v-select
          :items="statusOptions"
          :model-value="props.status"
          density="compact"
          hide-details
          label="Status"
          prepend-inner-icon="mdi-list-status"
          variant="outlined"
          @update:model-value="
            emit(
              'update:status',
              ($event as AdminQuotaResetEventStatusFilter | null) ?? 'all',
            )
          "
        />
        <v-btn
          color="primary"
          prepend-icon="mdi-refresh"
          rounded="xl"
          variant="tonal"
          :loading="props.loading"
          @click="emit('refresh')"
        >
          Refresh
        </v-btn>
      </div>
    </AppFilterCard>

    <v-alert v-if="props.error" type="warning" variant="tonal" rounded="lg">
      {{ props.error }}
    </v-alert>

    <AppSectionCard
      icon="mdi-book-clock-outline"
      title="Register entries"
      :subtitle="
        props.total === 1
          ? '1 event matches the current filters.'
          : `${formatNumber(props.total)} events match the current filters.`
      "
    >
      <AppServerDataTable
        default-sort-by="createdAt"
        default-sort-dir="desc"
        empty-icon="mdi-history"
        empty-title="No reset events found"
        empty-text="Try another period, event type, status, or target search."
        :headers="headers"
        :items="props.items"
        :loading="props.loading"
        :page="props.page"
        :page-size="props.pageSize"
        :sort-by="props.sortBy"
        :sort-dir="props.sortDir"
        :total="props.total"
        @update:page="emit('update:page', $event)"
        @update:page-size="emit('update:page-size', $event)"
        @update:sort="
          (nextSortBy, nextSortDir) =>
            emit('update:sort', nextSortBy, nextSortDir)
        "
      >
        <template #item.user="{ item }">
          <NuxtLink
            class="admin-quota-events-section__target"
            :to="targetLink(item)"
          >
            <v-icon
              :icon="
                item.target.accountType === 'service'
                  ? 'mdi-robot-outline'
                  : 'mdi-account-outline'
              "
              size="18"
            />
            <span>
              <strong>{{ item.target.displayName }}</strong>
              <small>{{ item.target.accountId }}</small>
            </span>
          </NuxtLink>
        </template>

        <template #item.type="{ item }">
          <span class="app-table-text app-table-text--strong">
            {{ typeLabel(item.type) }}
          </span>
        </template>

        <template #item.status="{ item }">
          <v-chip
            :color="item.status === 'pending' ? 'warning' : 'success'"
            label
            size="small"
            variant="tonal"
          >
            {{ item.status === 'pending' ? 'Pending' : 'Applied' }}
          </v-chip>
        </template>

        <template #item.balance="{ item }">
          <div class="admin-quota-events-section__stack">
            <strong>{{ signedDelta(item.delta) }}</strong>
            <span>Balance {{ formatNumber(item.balanceAfter) }}</span>
          </div>
        </template>

        <template #item.actor="{ item }">
          <div class="admin-quota-events-section__stack">
            <strong>{{ actorLabel(item.actor) }}</strong>
            <span>{{ actorCaption(item.actor) }}</span>
          </div>
        </template>

        <template #item.context="{ item }">
          <div class="admin-quota-events-section__stack">
            <span>Month {{ monthLabel(item.grantMonth) }}</span>
            <span>
              Cutoff
              {{
                item.resetCutoffAt ? formatDateTime(item.resetCutoffAt) : '—'
              }}
            </span>
          </div>
        </template>

        <template #item.createdAt="{ item }">
          <span class="app-table-text">{{
            formatDateTime(item.createdAt)
          }}</span>
        </template>

        <template #item.appliedAt="{ item }">
          <span class="app-table-text">
            {{
              item.appliedAt ? formatDateTime(item.appliedAt) : 'Not applied'
            }}
          </span>
        </template>

        <template #item.reconciliation="{ item }">
          <div class="admin-quota-events-section__stack">
            <strong>
              {{
                item.type !== 'quota_reset_used'
                  ? 'Not required'
                  : item.status === 'applied'
                    ? 'Reconciled'
                    : `${formatNumber(item.reconcileAttempts)} attempts`
              }}
            </strong>
            <span v-if="item.lastReconcileAttemptAt">
              Last {{ formatDateTime(item.lastReconcileAttemptAt) }}
            </span>
          </div>
        </template>
      </AppServerDataTable>
    </AppSectionCard>
  </section>
</template>

<style scoped>
.admin-quota-events-section {
  display: grid;
  gap: 14px;
}

.admin-quota-events-section__filters {
  display: grid;
  grid-template-columns:
    auto minmax(220px, 1fr) minmax(190px, 0.65fr)
    minmax(160px, 0.5fr) auto;
  align-items: center;
  gap: 12px;
}

.admin-quota-events-section__target {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: inherit;
  text-decoration: none;
}

.admin-quota-events-section__target > span,
.admin-quota-events-section__stack {
  display: grid;
  gap: 2px;
}

.admin-quota-events-section__target small,
.admin-quota-events-section__stack span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.76rem;
}

@media (max-width: 1199px) {
  .admin-quota-events-section__filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 599px) {
  .admin-quota-events-section__filters {
    grid-template-columns: 1fr;
  }
}
</style>
