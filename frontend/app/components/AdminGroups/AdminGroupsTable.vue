<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { AccessGroup } from '~/types/groups'
import type { AppRowAction } from '~/types/row-actions'
import { formatNumber } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  items: AccessGroup[]
  loading: boolean
  page: number
  pageSize: number
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  create: []
  delete: [group: AccessGroup]
  edit: [group: AccessGroup]
  manageMembers: [group: AccessGroup]
  refresh: []
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name' },
  appTableCenteredColumn({
    title: 'Providers',
    key: 'providerCount',
  }),
  appTableCenteredColumn({
    title: 'Model rules',
    key: 'modelPatternCount',
  }),
  appTableCenteredColumn({
    title: 'Members',
    key: 'memberCount',
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
  }),
]

const rowActions: AppRowAction<AccessGroup>[] = [
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (group) => emit('edit', group),
    title: 'Edit group',
  },
  {
    icon: 'mdi-account-multiple-plus-outline',
    key: 'members',
    onSelect: (group) => emit('manageMembers', group),
    title: 'Manage members',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (group) => emit('delete', group),
    title: 'Delete group',
  },
]

const summaryLabel = computed(() => {
  if (props.total === 0) {
    return 'No groups match the current filters.'
  }
  return props.total === 1
    ? '1 access group configured.'
    : `${props.total} access groups configured.`
})

function displayName(group: AccessGroup) {
  return group.displayName || group.name
}
</script>

<template>
  <AppSectionCard
    icon="mdi-account-multiple-check-outline"
    title="Access groups"
    :subtitle="summaryLabel"
  >
    <template #actions>
      <v-btn
        color="primary"
        rounded="xl"
        prepend-icon="mdi-plus"
        @click="emit('create')"
      >
        Create group
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
      empty-icon="mdi-account-multiple-remove-outline"
      empty-title="No groups found"
      empty-text="Create a group to grant provider and model access."
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
        <div class="admin-groups-table__name-cell">
          <span class="app-table-text app-table-text--strong">
            {{ displayName(item) }}
          </span>
          <span class="app-table-text app-table-text--muted">
            {{ item.name }}
          </span>
        </div>
      </template>

      <template #item.providerCount="{ item }">
        <span class="app-table-text">{{
          formatNumber(item.providerCount)
        }}</span>
      </template>

      <template #item.modelPatternCount="{ item }">
        <span class="app-table-text">
          {{ formatNumber(item.modelPatternCount) }}
        </span>
      </template>

      <template #item.memberCount="{ item }">
        <span class="app-table-text">{{ formatNumber(item.memberCount) }}</span>
      </template>

      <template #item.actions="{ item }">
        <AppRowActionMenu :actions="rowActions" :item="item" min-width="210" />
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>

<style scoped>
.admin-groups-table__name-cell {
  display: grid;
  min-width: 0;
  gap: 2px;
}
</style>
