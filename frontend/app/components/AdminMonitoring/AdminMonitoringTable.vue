<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { MonitoringService, MonitoringStatus } from '~/types/monitoring'
import type { AppRowAction } from '~/types/row-actions'
import { formatDateTime, formatDurationMs } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  items: MonitoringService[]
  loading: boolean
  page: number
  pageSize: number
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  check: [service: MonitoringService]
  create: []
  delete: [service: MonitoringService]
  edit: [service: MonitoringService]
  refresh: []
  toggle: [service: MonitoringService]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name' },
  { title: 'URL', key: 'url' },
  appTableCenteredColumn({ title: 'Expected', key: 'expectedStatusCode' }),
  appTableCenteredColumn({ title: 'Interval', key: 'intervalSeconds' }),
  appTableCenteredColumn({ title: 'Status', key: 'status' }),
  appTableCenteredColumn({ title: 'Last check', key: 'lastCheckedAt' }),
  appTableCenteredColumn({ title: 'Latency', key: 'lastLatencyMs' }),
  appTableCenteredColumn({
    title: 'Enabled',
    key: 'enabled',
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
  }),
]

function displayName(service: MonitoringService) {
  return service.displayName || 'No display name'
}

function statusLabel(status: MonitoringStatus) {
  return status === 'degraded' ? 'Degraded' : 'OK'
}

function statusColor(service: MonitoringService) {
  if (!service.enabled) {
    return 'grey'
  }
  return service.status === 'degraded' ? 'warning' : 'success'
}

function intervalLabel(seconds: number) {
  if (seconds < 60) {
    return `${seconds}s`
  }
  if (seconds % 3600 === 0) {
    return `${seconds / 3600}h`
  }
  if (seconds % 60 === 0) {
    return `${seconds / 60}m`
  }
  return `${seconds}s`
}

const rowActions: AppRowAction<MonitoringService>[] = [
  {
    icon: 'mdi-play-circle-outline',
    key: 'check',
    onSelect: (service) => emit('check', service),
    title: 'Run check',
  },
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (service) => emit('edit', service),
    title: 'Edit service',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (service) => emit('delete', service),
    title: 'Delete service',
  },
]
</script>

<template>
  <AppSectionCard
    icon="mdi-heart-pulse"
    title="Monitoring services"
    subtitle="HTTP/S services checked by the scheduler for user-facing incident banners."
  >
    <template #actions>
      <v-btn
        color="primary"
        rounded="xl"
        prepend-icon="mdi-plus"
        @click="emit('create')"
      >
        Create service
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
      empty-icon="mdi-heart-broken-outline"
      empty-title="No monitoring services found"
      empty-text="Create a service to start health checks."
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
        <div class="admin-monitoring-table__identity">
          <span class="app-table-text app-table-text--strong">
            {{ item.name }}
          </span>
          <span class="app-table-text app-table-text--secondary">
            {{ displayName(item) }}
          </span>
        </div>
      </template>

      <template #item.url="{ item }">
        <span
          class="app-table-text app-table-text--secondary"
          :title="item.url"
        >
          {{ item.url }}
        </span>
      </template>

      <template #item.expectedStatusCode="{ item }">
        <div class="app-table-center">
          <v-chip size="small" label variant="tonal" color="primary">
            {{ item.expectedStatusCode }}
          </v-chip>
        </div>
      </template>

      <template #item.intervalSeconds="{ item }">
        <div class="app-table-center">
          {{ intervalLabel(item.intervalSeconds) }}
        </div>
      </template>

      <template #item.status="{ item }">
        <div class="app-table-center">
          <v-chip size="small" label variant="tonal" :color="statusColor(item)">
            {{ item.enabled ? statusLabel(item.status) : 'Disabled' }}
          </v-chip>
        </div>
      </template>

      <template #item.lastCheckedAt="{ item }">
        <div class="app-table-center">
          {{ formatDateTime(item.lastCheckedAt) }}
        </div>
      </template>

      <template #item.lastLatencyMs="{ item }">
        <div class="app-table-center">
          {{
            item.lastCheckedAt
              ? formatDurationMs(item.lastLatencyMs)
              : 'Pending'
          }}
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

      <template #item.actions="{ item }">
        <AppRowActionMenu :actions="rowActions" :item="item" min-width="200" />
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>

<style scoped>
.admin-monitoring-table__identity {
  display: grid;
  gap: 2px;
  min-width: 0;
}
</style>
