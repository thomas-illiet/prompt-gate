<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { AppRowAction } from '~/types/row-actions'
import type { SubscriptionPlan } from '~/types/subscriptions'
import { formatNumber } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  items: SubscriptionPlan[]
  loading: boolean
  page: number
  pageSize: number
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  create: []
  delete: [plan: SubscriptionPlan]
  edit: [plan: SubscriptionPlan]
  refresh: []
  setDefault: [plan: SubscriptionPlan]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name' },
  { title: 'Description', key: 'description', sortable: false },
  appTableCenteredColumn({
    title: 'Accounts',
    key: 'assignedAccountsCount',
    sortable: false,
  }),
  appTableCenteredColumn({
    title: '5h quota',
    key: 'quota5hTokens',
  }),
  appTableCenteredColumn({
    title: '7d quota',
    key: 'quota7dTokens',
  }),
  appTableCenteredColumn({
    title: 'Default',
    key: 'isDefault',
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
  }),
]

const summaryLabel = computed(() => {
  if (props.total === 0) {
    return 'No subscription plans configured.'
  }
  return props.total === 1
    ? '1 subscription plan configured.'
    : `${props.total} subscription plans configured.`
})

const rowActions: AppRowAction<SubscriptionPlan>[] = [
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (plan) => emit('edit', plan),
    title: 'Edit plan',
  },
  {
    icon: 'mdi-star-check-outline',
    key: 'default',
    onSelect: (plan) => emit('setDefault', plan),
    title: 'Set as default',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (plan) => emit('delete', plan),
    title: 'Delete plan',
  },
]

function quotaLabel(value: number | null) {
  return value == null ? 'Unlimited' : formatNumber(value)
}

function assignedAccountsLabel(plan: SubscriptionPlan) {
  const count = plan.assignedAccountsCount
  return count === 1 ? '1 account' : `${formatNumber(count)} accounts`
}
</script>

<template>
  <AppSectionCard
    icon="mdi-card-account-details-star-outline"
    title="Subscription plans"
    :subtitle="summaryLabel"
  >
    <template #actions>
      <v-btn
        color="primary"
        rounded="xl"
        prepend-icon="mdi-plus"
        @click="emit('create')"
      >
        Create plan
      </v-btn>

      <v-btn
        color="primary"
        variant="tonal"
        rounded="xl"
        prepend-icon="mdi-refresh"
        :loading="props.loading"
        @click="emit('refresh')"
      >
        Refresh
      </v-btn>
    </template>

    <AppServerDataTable
      default-sort-by="name"
      default-sort-dir="asc"
      empty-icon="mdi-card-account-details-outline"
      empty-title="No subscription plans found"
      empty-text="Create a plan before users can access the proxy."
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
      <template #item.name="{ item }">
        <span class="app-table-text app-table-text--strong">
          {{ item.name }}
        </span>
      </template>

      <template #item.description="{ item }">
        <span class="app-table-text app-table-text--secondary">
          {{ item.description || 'No description' }}
        </span>
      </template>

      <template #item.assignedAccountsCount="{ item }">
        <span class="app-table-text app-table-text--strong">
          {{ assignedAccountsLabel(item) }}
        </span>
      </template>

      <template #item.quota5hTokens="{ item }">
        <span class="app-table-text">
          {{ quotaLabel(item.quota5hTokens) }}
        </span>
      </template>

      <template #item.quota7dTokens="{ item }">
        <span class="app-table-text">
          {{ quotaLabel(item.quota7dTokens) }}
        </span>
      </template>

      <template #item.isDefault="{ item }">
        <div class="app-table-center">
          <v-chip
            size="small"
            label
            variant="tonal"
            :color="item.isDefault ? 'success' : 'default'"
          >
            {{ item.isDefault ? 'Default' : 'Manual' }}
          </v-chip>
        </div>
      </template>

      <template #item.actions="{ item }">
        <AppRowActionMenu :actions="rowActions" :item="item" min-width="190" />
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>
