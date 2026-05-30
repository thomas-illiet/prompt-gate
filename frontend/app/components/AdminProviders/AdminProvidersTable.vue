<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { AppRowAction } from '~/types/row-actions'
import type { Provider, ProviderType } from '~/types/providers'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  items: Provider[]
  loading: boolean
  page: number
  pageSize: number
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  create: []
  delete: [provider: Provider]
  edit: [provider: Provider]
  refresh: []
  toggle: [provider: Provider]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name' },
  {
    title: 'Display name',
    key: 'displayName',
  },
  appTableCenteredColumn({
    title: 'Type',
    key: 'type',
  }),
  appTableCenteredColumn({
    title: 'Virtual key',
    key: 'hasApiKey',
  }),
  appTableCenteredColumn({
    title: 'Status',
    key: 'enabled',
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
  }),
]

// providerTypeLabel returns the display label for a provider type.
function providerTypeLabel(type: ProviderType) {
  switch (type) {
    case 'anthropic':
      return 'Anthropic'
    case 'ollama':
      return 'Ollama'
    default:
      return 'OpenAI'
  }
}

// providerTypeColor returns the Vuetify color for a provider type.
function providerTypeColor(type: ProviderType) {
  switch (type) {
    case 'anthropic':
      return 'deep-purple'
    case 'ollama':
      return 'teal'
    default:
      return 'primary'
  }
}

// displayName returns the preferred provider display name.
function displayName(provider: Provider) {
  return provider.displayName || 'No display name'
}

const rowActions: AppRowAction<Provider>[] = [
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (provider) => emit('edit', provider),
    title: 'Edit provider',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (provider) => emit('delete', provider),
    title: 'Delete provider',
  },
]
</script>

<template>
  <AppSectionCard
    icon="mdi-cloud-outline"
    title="Providers"
    subtitle="Providers define upstream LLM endpoints available through the proxy."
  >
    <template #actions>
      <v-btn
        color="primary"
        rounded="xl"
        prepend-icon="mdi-plus"
        @click="emit('create')"
      >
        Create provider
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
      empty-icon="mdi-cloud-off-outline"
      empty-title="No providers found"
      empty-text="Create a provider to make models available."
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

      <template #item.displayName="{ item }">
        <span
          class="app-table-text app-table-text--secondary"
          :title="displayName(item)"
        >
          {{ displayName(item) }}
        </span>
      </template>

      <template #item.type="{ item }">
        <div class="app-table-center">
          <v-chip
            size="small"
            label
            variant="tonal"
            :color="providerTypeColor(item.type)"
          >
            {{ providerTypeLabel(item.type) }}
          </v-chip>
        </div>
      </template>

      <template #item.hasApiKey="{ item }">
        <div class="app-table-center">
          <v-chip
            size="small"
            label
            variant="tonal"
            :color="item.hasApiKey ? 'success' : 'grey'"
          >
            {{ item.hasApiKey ? 'Saved' : 'Missing' }}
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

      <template #item.actions="{ item }">
        <AppRowActionMenu :actions="rowActions" :item="item" min-width="200" />
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>
