<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'
import type { FAQEntry } from '~/types/faq'
import type { AppRowAction } from '~/types/row-actions'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{ items: FAQEntry[]; loading: boolean; page: number; pageSize: number; sortBy: string; sortDir: 'asc' | 'desc'; total: number }>()
const emit = defineEmits<{
  create: []; delete: [entry: FAQEntry]; edit: [entry: FAQEntry]; move: [entry: FAQEntry, position: number]; refresh: []; toggle: [entry: FAQEntry]
  'update:page': [value: number]; 'update:page-size': [value: number]; 'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()
const headers: DataTableHeader[] = [
  appTableCenteredColumn({ title: 'Order', key: 'position' }),
  { title: 'Question', key: 'question' },
  appTableCenteredColumn({ title: 'Published', key: 'published' }),
  appTableCenteredColumn({ title: 'Actions', key: 'actions', sortable: false }),
]
const rowActions: AppRowAction<FAQEntry>[] = [
  { key: 'edit', icon: 'mdi-pencil-outline', title: 'Edit entry', onSelect: (entry) => emit('edit', entry) },
  { key: 'delete', icon: 'mdi-delete-outline', color: 'error', title: 'Delete entry', onSelect: (entry) => emit('delete', entry) },
]
</script>

<template>
  <AppSectionCard icon="mdi-frequently-asked-questions" title="FAQ entries" subtitle="Create, order, preview, and publish user documentation.">
    <template #actions>
      <v-btn color="primary" prepend-icon="mdi-plus" rounded="xl" @click="emit('create')">Create entry</v-btn>
      <v-btn color="primary" prepend-icon="mdi-refresh" rounded="xl" variant="tonal" :loading="props.loading" @click="emit('refresh')">Refresh</v-btn>
    </template>
    <AppServerDataTable default-sort-by="position" default-sort-dir="asc" empty-icon="mdi-help-box-outline" empty-title="No FAQ entries" empty-text="Create the first documentation entry." :headers="headers" :items="props.items" :loading="props.loading" :page="props.page" :page-size="props.pageSize" :sort-by="props.sortBy" :sort-dir="props.sortDir" :total="props.total" @update:page="emit('update:page', $event)" @update:page-size="emit('update:page-size', $event)" @update:sort="(by, dir) => emit('update:sort', by, dir)">
      <template #item.position="{ item }">
        <div class="app-table-center">
          <v-btn density="compact" icon="mdi-chevron-up" size="small" variant="text" :disabled="item.position === 0" @click="emit('move', item, item.position - 1)" />
          <span>{{ item.position + 1 }}</span>
          <v-btn density="compact" icon="mdi-chevron-down" size="small" variant="text" :disabled="item.position >= props.total - 1" @click="emit('move', item, item.position + 1)" />
        </div>
      </template>
      <template #item.question="{ item }"><span class="app-table-text app-table-text--strong">{{ item.question }}</span></template>
      <template #item.published="{ item }"><div class="app-table-center"><AppStatusToggleButton :active="item.published" @click="emit('toggle', item)" /></div></template>
      <template #item.actions="{ item }"><AppRowActionMenu :actions="rowActions" :item="item" /></template>
    </AppServerDataTable>
  </AppSectionCard>
</template>
