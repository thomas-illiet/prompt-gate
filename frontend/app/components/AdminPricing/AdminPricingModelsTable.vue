<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { ModelPriceRecord } from '~/types/pricing'
import type { AppRowAction } from '~/types/row-actions'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  items: ModelPriceRecord[]
  loading: boolean
}>()

const emit = defineEmits<{
  create: []
  delete: [price: ModelPriceRecord]
  edit: [price: ModelPriceRecord]
  refresh: []
}>()

const headers: DataTableHeader[] = [
  { title: 'Provider', key: 'providerName' },
  { title: 'Model', key: 'model' },
  appTableCenteredColumn({ title: 'Input USD / 1M', key: 'input' }),
  appTableCenteredColumn({ title: 'Output USD / 1M', key: 'output' }),
  appTableCenteredColumn({ title: 'Actions', key: 'actions', sortable: false }),
]

const rowActions: AppRowAction<ModelPriceRecord>[] = [
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (price) => emit('edit', price),
    title: 'Edit price',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (price) => emit('delete', price),
    title: 'Delete price',
  },
]

function formatPrice(value: number) {
  return new Intl.NumberFormat('en-US', {
    maximumFractionDigits: 6,
    minimumFractionDigits: 0,
  }).format(value)
}
</script>

<template>
  <AppSectionCard
    icon="mdi-robot-outline"
    title="Model prices"
    subtitle="Model-specific USD rates override fallback pricing for cost estimates."
  >
    <template #actions>
      <v-btn
        color="primary"
        rounded="xl"
        prepend-icon="mdi-plus"
        @click="emit('create')"
      >
        Create price
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

    <v-data-table
      class="app-server-data-table"
      item-value="id"
      :headers="headers"
      :items="props.items"
      :loading="props.loading"
      :items-per-page="10"
    >
      <template #no-data>
        <AppEmptyState
          compact
          icon="mdi-currency-usd-off"
          title="No model prices"
          text="Create a model price or use the check alert to add missing configurations."
          tone="warning"
        />
      </template>

      <template #item.providerName="{ item }">
        <span class="app-table-text app-table-text--strong">
          {{ item.providerName }}
        </span>
      </template>

      <template #item.model="{ item }">
        <span class="app-table-text app-table-text--secondary">
          {{ item.model }}
        </span>
      </template>

      <template #item.input="{ item }">
        <div class="app-table-center">
          {{ formatPrice(item.input) }}
        </div>
      </template>

      <template #item.output="{ item }">
        <div class="app-table-center">
          {{ formatPrice(item.output) }}
        </div>
      </template>

      <template #item.actions="{ item }">
        <AppRowActionMenu :actions="rowActions" :item="item" min-width="180" />
      </template>
    </v-data-table>
  </AppSectionCard>
</template>
