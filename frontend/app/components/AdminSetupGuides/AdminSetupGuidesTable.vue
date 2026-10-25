<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { AppRowAction } from '~/types/row-actions'
import type { SetupGuide } from '~/types/setup-guides'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  guides: SetupGuide[]
  loading: boolean
}>()

const emit = defineEmits<{
  edit: [guide: SetupGuide]
  remove: [guide: SetupGuide]
  reorder: [ids: string[]]
}>()

const headers: DataTableHeader[] = [
  appTableCenteredColumn({ title: 'Order', key: 'position', sortable: false }),
  { title: 'Guide', key: 'title', sortable: false },
  appTableCenteredColumn({
    title: 'Compatibility',
    key: 'compatibility',
    sortable: false,
  }),
  appTableCenteredColumn({
    title: 'Models',
    key: 'modelMode',
    sortable: false,
  }),
  appTableCenteredColumn({ title: 'Status', key: 'enabled', sortable: false }),
  appTableCenteredColumn({ title: 'Actions', key: 'actions', sortable: false }),
]

const rowActions: AppRowAction<SetupGuide>[] = [
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (guide) => emit('edit', guide),
    title: 'Edit guide',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (guide) => emit('remove', guide),
    title: 'Delete guide',
  },
]

function move(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= props.guides.length) return

  const ids = props.guides.map((guide) => guide.id)
  ;[ids[index], ids[target]] = [ids[target]!, ids[index]!]
  emit('reorder', ids)
}
</script>

<template>
  <v-data-table
    class="app-server-data-table"
    item-value="id"
    :aria-busy="props.loading"
    :headers="headers"
    :items="props.guides"
    :items-per-page="-1"
    :loading="props.loading"
    hide-default-footer
  >
    <template #loading>
      <v-skeleton-loader class="pa-4" type="table-row@4" />
    </template>

    <template #no-data>
      <AppEmptyState
        compact
        icon="mdi-file-code-outline"
        title="No setup guides"
        text="Create the first client guide and turn setup into a guided launch."
      />
    </template>

    <template #item.position="{ item, index }">
      <div class="app-table-center admin-setup-guides-table__order">
        <v-btn
          density="compact"
          icon="mdi-chevron-up"
          size="small"
          variant="text"
          :aria-label="`Move ${item.title} up`"
          :disabled="index === 0 || props.loading"
          @click.stop="move(index, -1)"
        />
        <span class="app-table-text app-table-text--muted">
          {{ index + 1 }}
        </span>
        <v-btn
          density="compact"
          icon="mdi-chevron-down"
          size="small"
          variant="text"
          :aria-label="`Move ${item.title} down`"
          :disabled="index === props.guides.length - 1 || props.loading"
          @click.stop="move(index, 1)"
        />
      </div>
    </template>

    <template #item.title="{ item }">
      <div class="admin-setup-guides-table__identity">
        <v-avatar color="primary" size="34" variant="tonal">
          <v-icon :icon="item.icon" size="19" />
        </v-avatar>
        <div class="admin-setup-guides-table__identity-copy">
          <span class="app-table-text app-table-text--strong">
            {{ item.title }}
          </span>
          <span class="app-table-text app-table-text--muted">
            {{ item.identifier }}
          </span>
        </div>
      </div>
    </template>

    <template #item.compatibility="{ item }">
      <div class="app-table-center">
        <span class="app-table-text app-table-text--secondary">
          {{ item.compatibility }}
        </span>
      </div>
    </template>

    <template #item.modelMode="{ item }">
      <div class="app-table-center">
        <span class="app-table-text app-table-text--secondary">
          {{ item.modelMode }}
        </span>
      </div>
    </template>

    <template #item.enabled="{ item }">
      <div class="app-table-center">
        <v-chip
          :color="item.enabled ? 'success' : undefined"
          :prepend-icon="
            item.enabled
              ? 'mdi-check-circle-outline'
              : 'mdi-pause-circle-outline'
          "
          label
          size="small"
          variant="tonal"
        >
          {{ item.enabled ? 'Enabled' : 'Disabled' }}
        </v-chip>
      </div>
    </template>

    <template #item.actions="{ item }">
      <AppRowActionMenu :actions="rowActions" :item="item" />
    </template>
  </v-data-table>
</template>

<style scoped>
.admin-setup-guides-table__order {
  flex-wrap: nowrap;
  gap: 2px;
}

.admin-setup-guides-table__identity {
  display: flex;
  min-width: 180px;
  align-items: center;
  gap: 10px;
}

.admin-setup-guides-table__identity-copy {
  display: grid;
  min-width: 0;
  gap: 2px;
}

@media (max-width: 600px) {
  :deep(.v-data-table-rows-no-data .app-empty-state) {
    position: sticky;
    left: 16px;
    width: calc(100vw - 64px);
    max-width: calc(100vw - 64px);
    box-sizing: border-box;
  }
}
</style>
