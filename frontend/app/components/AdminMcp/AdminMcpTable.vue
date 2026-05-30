<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { MCPServer } from '~/types/mcp'
import type { AppRowAction } from '~/types/row-actions'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  items: MCPServer[]
  loading: boolean
  page: number
  pageSize: number
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  create: []
  delete: [server: MCPServer]
  edit: [server: MCPServer]
  refresh: []
  toggle: [server: MCPServer]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name', width: 150 },
  {
    title: 'Display name',
    key: 'displayName',
    width: 150,
  },
  { title: 'URL', key: 'url', width: 250 },
  appTableCenteredColumn({
    title: 'Headers',
    key: 'headers',
    width: 96,
  }),
  {
    title: 'Tool filters',
    key: 'filters',
    width: 160,
  },
  appTableCenteredColumn({
    title: 'Status',
    key: 'enabled',
    width: 96,
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
    width: 96,
  }),
]

// displayName returns the preferred MCP server display name.
function displayName(server: MCPServer) {
  return server.displayName || 'No display name'
}

// headersLabel summarizes configured MCP headers for table display.
function headersLabel(server: MCPServer) {
  const total = server.headers.length
  if (total === 0) {
    return 'None'
  }

  return total === 1 ? '1 header' : `${total} headers`
}

// filtersLabel summarizes MCP allow and deny filters for table display.
function filtersLabel(server: MCPServer) {
  const filters = []
  if (server.allowPattern) {
    filters.push(`allow: ${server.allowPattern}`)
  }
  if (server.denyPattern) {
    filters.push(`deny: ${server.denyPattern}`)
  }

  return filters.length > 0 ? filters.join(' / ') : 'No filters'
}

const rowActions: AppRowAction<MCPServer>[] = [
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (server) => emit('edit', server),
    title: 'Edit server',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (server) => emit('delete', server),
    title: 'Delete server',
  },
]

</script>

<template>
  <AppSectionCard
    icon="mdi-server-network"
    title="MCP servers"
    subtitle="Enabled servers are injected into the proxy runtime after config reload."
  >
    <template #actions>
      <v-btn
        color="primary"
        rounded="xl"
        prepend-icon="mdi-plus"
        @click="emit('create')"
      >
        Create server
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
      empty-icon="mdi-server-off"
      empty-title="No MCP servers found"
      empty-text="Create a server to expose tools."
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
        <span class="app-table-text app-table-text--strong admin-mcp-table__name">
          {{ item.name }}
        </span>
      </template>

      <template #item.displayName="{ item }">
        <span
          class="app-table-text app-table-text--secondary admin-mcp-table__name"
          :title="displayName(item)"
        >
          {{ displayName(item) }}
        </span>
      </template>

      <template #item.url="{ item }">
        <span
          class="app-table-text app-table-text--secondary admin-mcp-table__url"
          :title="item.url"
        >
          {{ item.url }}
        </span>
      </template>

      <template #item.headers="{ item }">
        <div class="app-table-center">
          <v-chip size="small" label variant="tonal" color="primary">
            {{ headersLabel(item) }}
          </v-chip>
        </div>
      </template>

      <template #item.filters="{ item }">
        <span
          class="app-table-text app-table-text--secondary admin-mcp-table__filters"
          :title="filtersLabel(item)"
        >
          {{ filtersLabel(item) }}
        </span>
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
        <AppRowActionMenu :actions="rowActions" :item="item" />
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>

<style scoped>
.admin-mcp-table__name {
  max-width: min(14vw, 132px);
}

.admin-mcp-table__url {
  max-width: min(22vw, 230px);
}

.admin-mcp-table__filters {
  max-width: min(15vw, 148px);
}
</style>
