<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type {
  AdminQuotaAccountTypeFilter,
  AdminQuotaStateItem,
  AdminQuotaStatesView,
  AdminQuotaWindow,
} from '~/types/quota-resets'
import { formatDateTime, formatNumber } from '~/utils/formatters'

const props = defineProps<{
  accountType: AdminQuotaAccountTypeFilter
  error: string | null
  items: AdminQuotaStateItem[]
  loading: boolean
  page: number
  pageSize: number
  projectionSyncedAt: string | null
  search: string
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
  view: AdminQuotaStatesView
}>()

const emit = defineEmits<{
  refresh: []
  'update:account-type': [value: AdminQuotaAccountTypeFilter]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:search': [value: string]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
  'update:view': [value: AdminQuotaStatesView]
}>()

const headers: DataTableHeader[] = [
  { title: 'Account', key: 'displayName' },
  { title: 'Status', key: 'saturation' },
  { title: '5-hour window', key: 'window5h', sortable: false },
  { title: '7-day window', key: 'window7d', sortable: false },
  { title: 'Next reset', key: 'resetAt' },
  { title: 'Projection freshness', key: 'syncedAt' },
]

const accountTypeOptions = [
  { title: 'All account types', value: 'all' },
  { title: 'Users', value: 'user' },
  { title: 'Service accounts', value: 'service' },
]

function accountLink(item: AdminQuotaStateItem) {
  return {
    path:
      item.accountType === 'service'
        ? '/admin/service-accounts'
        : '/admin/users',
    query: { search: item.accountId },
  }
}

function quotaWindow(item: AdminQuotaStateItem, window: AdminQuotaWindow) {
  return item.windows.find((candidate) => candidate.window === window) ?? null
}

function percentLabel(percent: number | null) {
  if (percent == null) return 'Unlimited'
  return `${new Intl.NumberFormat('en-US', {
    maximumFractionDigits: 1,
  }).format(percent)}%`
}

function windowValueLabel(item: AdminQuotaStateItem, window: AdminQuotaWindow) {
  if (item.status === 'unassigned') return 'Unassigned'
  return percentLabel(quotaWindow(item, window)?.percent ?? null)
}

function windowCaption(item: AdminQuotaStateItem, window: AdminQuotaWindow) {
  if (item.status === 'unassigned') return 'No active plan'
  const state = quotaWindow(item, window)
  if (state?.quotaTokens == null) return 'No token ceiling'
  return `${formatNumber(state.usedTokens)} / ${formatNumber(state.quotaTokens)}`
}

function statusLabel(item: AdminQuotaStateItem) {
  if (item.status === 'unassigned') return 'Unassigned'
  return item.status[0]?.toUpperCase() + item.status.slice(1)
}

function statusColor(item: AdminQuotaStateItem) {
  if (item.status === 'critical' || item.status === 'unassigned') return 'error'
  if (item.status === 'warning') return 'warning'
  if (item.status === 'healthy') return 'success'
  return 'primary'
}
</script>

<template>
  <section class="admin-quota-states-section">
    <AppFilterCard
      icon="mdi-gauge"
      title="Live quota windows"
      subtitle="By default, only accounts at or above 80% saturation are shown. All includes every active SQL projection."
    >
      <div class="admin-quota-states-section__filters">
        <v-btn-toggle
          :model-value="props.view"
          aria-label="Quota projection view"
          color="primary"
          density="compact"
          mandatory
          variant="outlined"
          @update:model-value="
            emit('update:view', $event as AdminQuotaStatesView)
          "
        >
          <v-btn value="at-risk">At risk</v-btn>
          <v-btn value="all">All</v-btn>
        </v-btn-toggle>
        <v-text-field
          :model-value="props.search"
          clearable
          density="compact"
          hide-details
          label="Search accounts"
          placeholder="Name or identifier"
          prepend-inner-icon="mdi-magnify"
          variant="outlined"
          @update:model-value="
            emit('update:search', ($event as string | null) ?? '')
          "
        />
        <v-select
          :items="accountTypeOptions"
          :model-value="props.accountType"
          density="compact"
          hide-details
          label="Account type"
          prepend-inner-icon="mdi-account-switch-outline"
          variant="outlined"
          @update:model-value="
            emit(
              'update:account-type',
              ($event as AdminQuotaAccountTypeFilter | null) ?? 'all',
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
      icon="mdi-table-clock"
      title="Projected accounts"
      :subtitle="
        props.projectionSyncedAt
          ? `Projection snapshot ${formatDateTime(props.projectionSyncedAt)}`
          : 'Projection snapshot time unavailable'
      "
    >
      <AppServerDataTable
        default-sort-by="saturation"
        default-sort-dir="desc"
        empty-icon="mdi-check-circle-outline"
        empty-title="No matching quota projections"
        empty-text="No active projection matches the current risk, account, and search filters."
        :headers="headers"
        item-value="accountId"
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
        <template #item.displayName="{ item }">
          <NuxtLink
            class="admin-quota-states-section__account"
            :to="accountLink(item)"
          >
            <v-icon
              :icon="
                item.accountType === 'service'
                  ? 'mdi-robot-outline'
                  : 'mdi-account-outline'
              "
              size="18"
            />
            <span>
              <strong>{{ item.displayName }}</strong>
              <small>{{ item.planName || 'No subscription plan' }}</small>
            </span>
          </NuxtLink>
        </template>

        <template #item.saturation="{ item }">
          <v-chip :color="statusColor(item)" label size="small" variant="tonal">
            {{ statusLabel(item) }}
            <span v-if="item.maximumPercent != null">
              · {{ percentLabel(item.maximumPercent) }}
            </span>
          </v-chip>
        </template>

        <template
          v-for="window in ['5h', '7d'] as const"
          :key="window"
          #[`item.window${window}`]="{ item }"
        >
          <div class="admin-quota-states-section__window">
            <strong>{{ windowValueLabel(item, window) }}</strong>
            <span>{{ windowCaption(item, window) }}</span>
          </div>
        </template>

        <template #item.resetAt="{ item }">
          <span class="app-table-text">
            {{ item.nextResetAt ? formatDateTime(item.nextResetAt) : 'None' }}
          </span>
        </template>

        <template #item.syncedAt="{ item }">
          <span class="app-table-text">
            {{ item.syncedAt ? formatDateTime(item.syncedAt) : 'Unavailable' }}
          </span>
        </template>
      </AppServerDataTable>
    </AppSectionCard>
  </section>
</template>

<style scoped>
.admin-quota-states-section {
  display: grid;
  gap: 14px;
}

.admin-quota-states-section__filters {
  display: grid;
  grid-template-columns: auto minmax(220px, 1fr) minmax(190px, 0.6fr) auto;
  align-items: center;
  gap: 12px;
}

.admin-quota-states-section__account {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: inherit;
  text-decoration: none;
}

.admin-quota-states-section__account > span,
.admin-quota-states-section__window {
  display: grid;
  gap: 2px;
}

.admin-quota-states-section__account small,
.admin-quota-states-section__window span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.76rem;
}

@media (max-width: 959px) {
  .admin-quota-states-section__filters {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 599px) {
  .admin-quota-states-section__filters {
    grid-template-columns: 1fr;
  }
}
</style>
