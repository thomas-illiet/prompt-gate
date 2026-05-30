<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { AppRowAction } from '~/types/row-actions'
import type { UserToken } from '~/types/user-service'
import { formatDateTime } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'
import {
  canRevokeUserToken,
  userTokenStatus,
  userTokenStatusColor,
  userTokenStatusLabel,
} from '~/utils/user-tokens'

const props = defineProps<{
  items: UserToken[]
  loading: boolean
  page: number
  pageSize: number
  saving: boolean
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  create: []
  refresh: []
  revoke: [token: UserToken]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name' },
  { title: 'Description', key: 'description' },
  appTableCenteredColumn({
    title: 'Status',
    key: 'status',
  }),
  appTableCenteredColumn({
    title: 'Created',
    key: 'createdAt',
  }),
  appTableCenteredColumn({
    title: 'Expires',
    key: 'expiresAt',
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
  }),
]

const summaryLabel = computed(() => {
  if (props.total === 0) {
    return 'No virtual key records.'
  }

  if (props.items.length === props.total) {
    return props.total === 1
      ? '1 virtual key record.'
      : `${props.total} virtual key records.`
  }

  return `${props.items.length} of ${props.total} virtual key records.`
})

const emptyState = computed(() =>
  props.total === 0
    ? {
        title: 'No virtual keys',
        text: 'Create a virtual key to enable CLI, SDK, or proxy access.',
      }
    : {
        title: 'No matching virtual keys',
        text: 'Adjust the search or status filter to see more virtual key records.',
      },
)

const rowActions: AppRowAction<UserToken>[] = [
  {
    color: 'error',
    disabled: (token) => !canRevokeUserToken(token) || props.saving,
    icon: 'mdi-key-remove',
    key: 'revoke',
    onSelect: (token) => emit('revoke', token),
    title: 'Revoke key',
  },
]
</script>

<template>
  <AppSectionCard
    icon="mdi-key-chain"
    title="Virtual keys"
    :subtitle="summaryLabel"
  >
    <template #actions>
      <v-btn
        color="primary"
        variant="flat"
        rounded="lg"
        prepend-icon="mdi-key-plus"
        :disabled="props.saving"
        @click="emit('create')"
      >
        New key
      </v-btn>

      <v-btn
        color="primary"
        variant="tonal"
        rounded="lg"
        prepend-icon="mdi-refresh"
        :loading="props.loading"
        @click="emit('refresh')"
      >
        Refresh
      </v-btn>
    </template>

    <AppServerDataTable
      default-sort-by="createdAt"
      default-sort-dir="desc"
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
      <template #no-data>
        <AppEmptyState
          icon="mdi-key-outline"
          :title="emptyState.title"
          :text="emptyState.text"
        >
          <template v-if="props.total === 0" #actions>
            <v-btn
              color="primary"
              variant="tonal"
              rounded="lg"
              prepend-icon="mdi-key-plus"
              :disabled="props.saving"
              @click="emit('create')"
            >
              New key
            </v-btn>
          </template>
        </AppEmptyState>
      </template>

      <template #item.name="{ item }">
        <span class="app-table-text">{{ item.name }}</span>
      </template>

      <template #item.description="{ item }">
        <span class="app-table-text">
          {{ item.description || 'No description' }}
        </span>
      </template>

      <template #item.status="{ item }">
        <div class="app-table-center">
          <v-chip
            size="small"
            label
            variant="tonal"
            :color="userTokenStatusColor(userTokenStatus(item))"
          >
            {{ userTokenStatusLabel(userTokenStatus(item)) }}
          </v-chip>
        </div>
      </template>

      <template #item.createdAt="{ item }">
        <span class="app-table-text">
          {{ formatDateTime(item.createdAt) }}
        </span>
      </template>

      <template #item.expiresAt="{ item }">
        <span class="app-table-text">
          {{ formatDateTime(item.expiresAt) }}
        </span>
      </template>

      <template #item.actions="{ item }">
        <AppRowActionMenu
          aria-label="Open virtual key actions"
          :actions="rowActions"
          :item="item"
        />
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>
