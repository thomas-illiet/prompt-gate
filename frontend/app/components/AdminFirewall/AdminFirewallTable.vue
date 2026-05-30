<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { FirewallMoveDirection, FirewallRule } from '~/types/firewall'
import type { AppRowAction } from '~/types/row-actions'
import { formatDateTime } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  items: FirewallRule[]
  loading: boolean
  page: number
  pageSize: number
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  create: []
  delete: [rule: FirewallRule]
  edit: [rule: FirewallRule]
  move: [rule: FirewallRule, direction: FirewallMoveDirection]
  refresh: []
  simulate: []
  toggle: [rule: FirewallRule]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers: DataTableHeader[] = [
  appTableCenteredColumn({
    title: 'Priority',
    key: 'priority',
  }),
  { title: 'Address', key: 'address' },
  {
    title: 'Description',
    key: 'description',
  },
  appTableCenteredColumn({
    title: 'Action',
    key: 'action',
  }),
  appTableCenteredColumn({
    title: 'Status',
    key: 'enabled',
  }),
  appTableCenteredColumn({
    title: 'Updated',
    key: 'updatedAt',
  }),
  appTableCenteredColumn({
    title: 'Order',
    key: 'order',
    sortable: false,
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
  }),
]

// actionColor returns the Vuetify color for a firewall action.
function actionColor(rule: FirewallRule) {
  return rule.action === 'allow' ? 'success' : 'error'
}

// actionLabel returns the display label for a firewall action.
function actionLabel(rule: FirewallRule) {
  return rule.action === 'allow' ? 'Allow' : 'Deny'
}

// displayDescription returns a readable firewall rule description.
function displayDescription(rule: FirewallRule) {
  return rule.description || 'No description'
}

const rowActions: AppRowAction<FirewallRule>[] = [
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (rule) => emit('edit', rule),
    title: 'Edit rule',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (rule) => emit('delete', rule),
    title: 'Delete rule',
  },
]

</script>

<template>
  <AppSectionCard
    icon="mdi-shield-lock-outline"
    title="Firewall rules"
    subtitle="Rules are evaluated by ascending priority. First match decides."
  >
    <template #actions>
      <v-btn
        color="primary"
        variant="tonal"
        rounded="xl"
        prepend-icon="mdi-radar"
        @click="emit('simulate')"
      >
        Simulate IP
      </v-btn>

      <v-btn
        color="primary"
        rounded="xl"
        prepend-icon="mdi-plus"
        @click="emit('create')"
      >
        Create rule
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
      default-sort-by="priority"
      default-sort-dir="asc"
      empty-icon="mdi-shield-off-outline"
      empty-title="No firewall rules found"
      empty-text="Create a rule or adjust the current filters."
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
      <template #item.priority="{ item }">
        <div class="app-table-center">
          <v-chip size="small" label variant="tonal" color="primary">
            {{ item.priority }}
          </v-chip>
        </div>
      </template>

      <template #item.address="{ item }">
        <span class="app-table-text app-table-text--strong">
          {{ item.address }}
        </span>
      </template>

      <template #item.description="{ item }">
        <span
          class="app-table-text app-table-text--secondary"
          :title="displayDescription(item)"
        >
          {{ displayDescription(item) }}
        </span>
      </template>

      <template #item.action="{ item }">
        <div class="app-table-center">
          <v-chip size="small" label variant="tonal" :color="actionColor(item)">
            {{ actionLabel(item) }}
          </v-chip>
        </div>
      </template>

      <template #item.enabled="{ item }">
        <div class="app-table-center">
          <AppStatusToggleButton
            :active="item.enabled"
            @click="emit('toggle', item)"
          />
        </div>
      </template>

      <template #item.updatedAt="{ item }">
        <div class="app-table-center">
          <span class="app-table-text app-table-text--secondary">
            {{ formatDateTime(item.updatedAt) }}
          </span>
        </div>
      </template>

      <template #item.order="{ item }">
        <div class="admin-firewall-table__order">
          <v-btn
            :aria-label="`Move ${item.address} to higher priority`"
            icon="mdi-arrow-up"
            size="small"
            variant="text"
            :disabled="item.priority <= 1"
            @click="emit('move', item, 'decrease')"
          />

          <v-btn
            :aria-label="`Move ${item.address} to lower priority`"
            icon="mdi-arrow-down"
            size="small"
            variant="text"
            :disabled="item.priority >= 9999"
            @click="emit('move', item, 'increase')"
          />
        </div>
      </template>

      <template #item.actions="{ item }">
        <AppRowActionMenu :actions="rowActions" :item="item" />
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>

<style scoped>
.admin-firewall-table__order {
  justify-content: center;
}

.admin-firewall-table__order {
  display: flex;
  align-items: center;
}
</style>
