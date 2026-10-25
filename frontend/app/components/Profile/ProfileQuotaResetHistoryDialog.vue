<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type {
  QuotaResetHistoryItem,
  QuotaResetHistorySortBy,
  QuotaResetHistoryType,
} from '~/types/quota-resets'
import { formatDateTime } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  error: string | null
  items: QuotaResetHistoryItem[]
  loading: boolean
  maximum: number
  page: number
  pageSize: number
  sortBy: QuotaResetHistorySortBy
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  refresh: []
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: QuotaResetHistorySortBy, sortDir: 'asc' | 'desc']
}>()

const isOpen = defineModel<boolean>({ default: false })

const headers: DataTableHeader[] = [
  appTableCenteredColumn({ title: 'Date', key: 'createdAt' }),
  appTableCenteredColumn({ title: 'Operation', key: 'type' }),
  appTableCenteredColumn({ title: 'Change', key: 'delta' }),
  appTableCenteredColumn({
    title: 'Balance',
    key: 'balanceAfter',
  }),
]

const subtitle = computed(() => {
  if (props.total === 0) return 'No quota reset activity yet.'
  return props.total === 1
    ? '1 history entry.'
    : `${props.total} history entries.`
})

function typeLabel(type: QuotaResetHistoryType) {
  switch (type) {
    case 'monthly_grant':
      return 'Monthly credit'
    case 'admin_grant':
      return 'Granted credit'
    case 'quota_reset_used':
      return 'Quota reset used'
  }
}

function typeColor(type: QuotaResetHistoryType) {
  return type === 'quota_reset_used' ? 'warning' : 'success'
}

function deltaLabel(delta: number) {
  return delta > 0 ? `+${delta}` : String(delta)
}

function updateSort(sortBy: string, sortDir: 'asc' | 'desc') {
  if (
    sortBy === 'createdAt' ||
    sortBy === 'type' ||
    sortBy === 'delta' ||
    sortBy === 'balanceAfter'
  ) {
    emit('update:sort', sortBy, sortDir)
  }
}
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    content-class="profile-quota-reset-history__content"
    icon="mdi-history"
    max-width="820"
    :subtitle="subtitle"
    title="Quota reset history"
  >
    <v-alert v-if="props.error" rounded="lg" type="warning" variant="tonal">
      <div class="profile-quota-reset-history__alert">
        <span>{{ props.error }}</span>
        <v-btn
          prepend-icon="mdi-refresh"
          size="small"
          variant="text"
          @click="emit('refresh')"
        >
          Retry
        </v-btn>
      </div>
    </v-alert>

    <AppServerDataTable
      default-sort-by="createdAt"
      default-sort-dir="desc"
      empty-icon="mdi-history"
      empty-text="Monthly credits, granted credits, and used resets will appear here."
      empty-title="No reset history"
      item-value="id"
      :headers="headers"
      :items="props.items"
      :items-per-page-options="[20, 50, 100]"
      :loading="props.loading"
      :page="props.page"
      :page-size="props.pageSize"
      :sort-by="props.sortBy"
      :sort-dir="props.sortDir"
      :total="props.total"
      @update:page="emit('update:page', $event)"
      @update:page-size="emit('update:page-size', $event)"
      @update:sort="updateSort"
    >
      <template #item.createdAt="{ item }">
        <div class="app-table-center">
          <span class="app-table-text">{{
            formatDateTime(item.createdAt)
          }}</span>
        </div>
      </template>
      <template #item.type="{ item }">
        <div class="app-table-center">
          <v-chip
            :color="typeColor(item.type)"
            label
            size="small"
            variant="tonal"
          >
            {{ typeLabel(item.type) }}
          </v-chip>
        </div>
      </template>
      <template #item.delta="{ item }">
        <div class="app-table-center">
          <strong
            class="profile-quota-reset-history__delta"
            :class="{
              'profile-quota-reset-history__delta--positive': item.delta > 0,
              'profile-quota-reset-history__delta--negative': item.delta < 0,
            }"
          >
            {{ deltaLabel(item.delta) }}
          </strong>
        </div>
      </template>
      <template #item.balanceAfter="{ item }">
        <div class="app-table-center">
          <span class="app-table-text">
            {{ item.balanceAfter }} / {{ props.maximum }}
          </span>
        </div>
      </template>
    </AppServerDataTable>

    <template #actions>
      <AppDialogCloseButton @click="isOpen = false" />
    </template>
  </AppDialogCard>
</template>

<style scoped>
.profile-quota-reset-history__content {
  display: grid;
  gap: 14px;
}

.profile-quota-reset-history__alert {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.profile-quota-reset-history__delta--positive {
  color: rgb(var(--v-theme-success));
}

.profile-quota-reset-history__delta--negative {
  color: rgb(var(--v-theme-warning));
}

@media (max-width: 600px) {
  .profile-quota-reset-history__alert {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
