<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type {
  AdminPromptHistoryItem,
  PromptHistoryItem,
} from '~/types/user-service'
import {
  formatDateTime,
  formatDurationMs,
  formatNumber,
} from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

type PromptHistoryTableItem = PromptHistoryItem | AdminPromptHistoryItem

const props = withDefaults(
  defineProps<{
    items: PromptHistoryTableItem[]
    loading: boolean
    page: number
    pageSize: number
    scopeLabel?: string
    showUser?: boolean
    sortBy: string
    sortDir: 'asc' | 'desc'
    title?: string
    total: number
  }>(),
  {
    scopeLabel: 'your history',
    showUser: false,
    title: 'Prompt history',
  },
)

const emit = defineEmits<{
  refresh: []
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers = computed<DataTableHeader[]>(() => {
  const baseHeaders: DataTableHeader[] = [{ title: 'Prompt', key: 'prompt' }]
  if (props.showUser) {
    baseHeaders.push({ title: 'User', key: 'userName' })
  }

  return [
    ...baseHeaders,
    { title: 'Provider', key: 'provider' },
    { title: 'Model', key: 'model' },
    appTableCenteredColumn({
      title: 'Input tokens',
      key: 'inputTokens',
    }),
    appTableCenteredColumn({
      title: 'Output tokens',
      key: 'outputTokens',
    }),
    appTableCenteredColumn({
      title: 'Duration',
      key: 'durationMs',
    }),
    appTableCenteredColumn({
      title: 'Created',
      key: 'createdAt',
    }),
  ]
})

const summaryLabel = computed(() => {
  if (props.total === 0) {
    return 'No prompts match the current filters.'
  }

  if (props.items.length === props.total) {
    return props.total === 1
      ? `1 prompt in ${props.scopeLabel}.`
      : `${props.total} prompts in ${props.scopeLabel}.`
  }

  return `Showing ${props.items.length} of ${props.total} prompts.`
})
const emptyState = computed(() => ({
  title: 'No prompts in view',
  text: `Send traffic through PromptGate and ${props.scopeLabel} will start filling in here.`,
}))

// userLabel returns the best available display identity for admin prompt rows.
function userLabel(item: PromptHistoryTableItem) {
  if (!('userId' in item)) {
    return ''
  }

  return item.userName || item.userPreferredUsername || item.userId
}
</script>

<template>
  <AppSectionCard
    icon="mdi-history"
    :title="props.title"
    :subtitle="summaryLabel"
  >
    <template #actions>
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
          icon="mdi-message-text-clock-outline"
          :title="emptyState.title"
          :text="emptyState.text"
        >
          <template #actions>
            <v-btn
              to="/help"
              color="primary"
              variant="tonal"
              rounded="lg"
              prepend-icon="mdi-help-circle-outline"
            >
              Open setup guide
            </v-btn>
          </template>
        </AppEmptyState>
      </template>

      <template #item.prompt="{ item }">
        <div class="prompt-history-table__prompt">
          {{ item.prompt }}
        </div>
      </template>

      <template #item.provider="{ item }">
        <v-chip size="small" label variant="tonal" color="primary">
          {{ item.provider }}
        </v-chip>
      </template>

      <template #item.userName="{ item }">
        <div class="prompt-history-table__user">
          <span class="app-table-text">{{ userLabel(item) }}</span>
        </div>
      </template>

      <template #item.model="{ item }">
        <span class="app-table-text">{{ item.model }}</span>
      </template>

      <template #item.inputTokens="{ item }">
        <span class="app-table-text">
          {{ formatNumber(item.inputTokens) }}
        </span>
      </template>

      <template #item.outputTokens="{ item }">
        <span class="app-table-text">
          {{ formatNumber(item.outputTokens) }}
        </span>
      </template>

      <template #item.durationMs="{ item }">
        <span class="app-table-text">
          {{ formatDurationMs(item.durationMs) }}
        </span>
      </template>

      <template #item.createdAt="{ item }">
        <span class="app-table-text">
          {{ formatDateTime(item.createdAt) }}
        </span>
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>

<style scoped>
.prompt-history-table__prompt {
  min-width: 280px;
  max-width: 720px;
  padding-block: 10px;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.prompt-history-table__user {
  min-width: 180px;
  display: grid;
  gap: 2px;
}
</style>
