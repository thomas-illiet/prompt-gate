<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { AppRowAction } from '~/types/row-actions'
import type { ServiceAccount } from '~/types/service-accounts'
import { formatNumber } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  items: ServiceAccount[]
  loading: boolean
  page: number
  pageSize: number
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  create: []
  delete: [account: ServiceAccount]
  edit: [account: ServiceAccount]
  manageFirewall: [account: ServiceAccount]
  manageTokens: [account: ServiceAccount]
  notes: [account: ServiceAccount]
  refresh: []
  toggleStatus: [account: ServiceAccount]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name' },
  { title: 'Identifier', key: 'identifier' },
  appTableCenteredColumn({
    title: 'Status',
    key: 'isActive',
  }),
  appTableCenteredColumn({
    title: 'Firewall',
    key: 'firewallOverrideEnabled',
    sortable: false,
  }),
  appTableCenteredColumn({
    title: 'Plan',
    key: 'subscriptionPlan',
    sortable: false,
  }),
  appTableCenteredColumn({
    title: 'Input tokens',
    key: 'inputTokens',
  }),
  appTableCenteredColumn({
    title: 'Output tokens',
    key: 'outputTokens',
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
  }),
]

const summaryLabel = computed(() => {
  if (props.items.length === 0) {
    return 'No service accounts configured.'
  }

  return props.items.length === 1
    ? '1 service account configured.'
    : `${props.items.length} service accounts configured.`
})

const rowActions: AppRowAction<ServiceAccount>[] = [
  {
    icon: 'mdi-shield-account-outline',
    key: 'manageFirewall',
    onSelect: (account) => emit('manageFirewall', account),
    title: 'Firewall',
  },
  {
    icon: 'mdi-key-chain',
    key: 'manageTokens',
    onSelect: (account) => emit('manageTokens', account),
    title: 'Virtual keys',
  },
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (account) => emit('edit', account),
    title: 'Edit account',
  },
  {
    icon: 'mdi-note-edit-outline',
    key: 'notes',
    onSelect: (account) => emit('notes', account),
    title: 'Notes',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (account) => emit('delete', account),
    title: 'Delete account',
  },
]

function planLabel(account: ServiceAccount) {
  return account.effectiveSubscriptionPlan?.name ?? 'No subscription'
}

function planCaption(account: ServiceAccount) {
  if (!account.effectiveSubscriptionPlan) {
    return 'Required'
  }
  return account.subscriptionPlanId ? 'Assigned' : 'Default'
}

function planColor(account: ServiceAccount) {
  if (!account.effectiveSubscriptionPlan) {
    return 'error'
  }
  return account.subscriptionPlanId ? 'primary' : 'success'
}
</script>

<template>
  <AppSectionCard
    icon="mdi-account-key-outline"
    title="Service account access"
    :subtitle="summaryLabel"
  >
    <template #actions>
      <v-btn
        color="primary"
        rounded="xl"
        prepend-icon="mdi-plus"
        @click.stop="emit('create')"
      >
        New service account
      </v-btn>
      <v-btn
        color="primary"
        variant="tonal"
        rounded="xl"
        prepend-icon="mdi-refresh"
        :loading="props.loading"
        @click.stop="emit('refresh')"
      >
        Refresh
      </v-btn>
    </template>

    <AppServerDataTable
      default-sort-by="createdAt"
      default-sort-dir="desc"
      empty-icon="mdi-account-key-outline"
      empty-title="No service accounts found"
      empty-text="Create one or adjust the current filters."
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

      <template #item.identifier="{ item }">
        <span class="app-table-text">
          {{ item.identifier }}
        </span>
      </template>

      <template #item.isActive="{ item }">
        <div class="app-table-center">
          <AppStatusToggleButton
            :active="item.isActive"
            active-label="Active"
            inactive-label="Inactive"
            @click="emit('toggleStatus', item)"
          />
        </div>
      </template>

      <template #item.firewallOverrideEnabled="{ item }">
        <div class="app-table-center">
          <v-chip
            size="small"
            label
            variant="tonal"
            :color="item.firewallOverrideEnabled ? 'success' : 'default'"
          >
            {{ item.firewallOverrideEnabled ? 'Override' : 'Global' }}
          </v-chip>
        </div>
      </template>

      <template #item.subscriptionPlan="{ item }">
        <div class="app-table-center">
          <v-chip
            size="small"
            label
            variant="tonal"
            :color="planColor(item)"
          >
            {{ planLabel(item) }}
            <span class="service-accounts-table__chip-caption">
              {{ planCaption(item) }}
            </span>
          </v-chip>
        </div>
      </template>

      <template #item.inputTokens="{ item }">
        <span class="app-table-text">
          {{ formatNumber(item.inputTokens) }}
        </span>
      </template>

      <template #item.outputTokens="{ item }">
        <span class="app-table-text">
          {{ formatNumber(item.outputTokens) }}
        </span>
      </template>

      <template #item.actions="{ item }">
        <AppRowActionMenu :actions="rowActions" :item="item" min-width="190" />
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>

<style scoped>
.service-accounts-table__chip-caption {
  margin-left: 6px;
  opacity: 0.68;
}
</style>
