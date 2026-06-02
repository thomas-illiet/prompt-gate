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
const PROMPT_PREVIEW_MAX_LENGTH = 220

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
  const baseHeaders: DataTableHeader[] = [
    { title: 'Prompt', key: 'prompt', width: 360 },
  ]
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
    appTableCenteredColumn({
      title: 'Actions',
      key: 'actions',
      sortable: false,
    }),
  ]
})

const selectedGraphItem = shallowRef<PromptHistoryTableItem | null>(null)
const graphDialogOpen = shallowRef(false)
const selectedGraphActorLabel = computed(() =>
  selectedGraphItem.value ? requestActorLabel(selectedGraphItem.value) : '',
)
const promptPreviewById = computed(() => {
  return new Map(
    props.items.map((item) => [item.id, compactPromptPreview(item.prompt)]),
  )
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

// compactPromptPreview keeps table rows scannable while preserving full prompts in the detail dialog.
function compactPromptPreview(prompt: string) {
  const normalizedPrompt = prompt.replace(/\s+/g, ' ').trim()
  if (!normalizedPrompt) {
    return 'Empty prompt'
  }

  if (normalizedPrompt.length <= PROMPT_PREVIEW_MAX_LENGTH) {
    return normalizedPrompt
  }

  return `${normalizedPrompt.slice(0, PROMPT_PREVIEW_MAX_LENGTH).trimEnd()}...`
}

function promptPreview(item: PromptHistoryTableItem) {
  return (
    promptPreviewById.value.get(item.id) ?? compactPromptPreview(item.prompt)
  )
}

// requestActorLabel returns the graph's requester node label for each scope.
function requestActorLabel(item: PromptHistoryTableItem) {
  if (!props.showUser) {
    return 'You'
  }

  return userLabel(item) || 'Unknown user'
}

// openRequestGraph selects the row displayed in the request graph dialog.
function openRequestGraph(item: PromptHistoryTableItem) {
  selectedGraphItem.value = item
  graphDialogOpen.value = true
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
          <span
            class="prompt-history-table__prompt-preview"
            data-test="prompt-preview"
          >
            {{ promptPreview(item) }}
          </span>
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

      <template #item.actions="{ item }">
        <div class="app-table-center">
          <v-tooltip text="View request graph">
            <template #activator="{ props: tooltipProps }">
              <v-btn
                v-bind="tooltipProps"
                aria-label="View request graph"
                color="primary"
                icon="mdi-transit-connection-variant"
                size="small"
                variant="text"
                @click="openRequestGraph(item)"
              />
            </template>
          </v-tooltip>
        </div>
      </template>
    </AppServerDataTable>

    <PromptHistoryRequestGraphDialog
      v-model="graphDialogOpen"
      :actor-label="selectedGraphActorLabel"
      :prompt="selectedGraphItem"
    />
  </AppSectionCard>
</template>

<style scoped>
.prompt-history-table__prompt {
  width: clamp(220px, 28vw, 420px);
  max-width: clamp(220px, 28vw, 420px);
  padding-block: 8px;
}

.prompt-history-table__prompt-preview {
  display: -webkit-box;
  max-height: calc(1.35em * 2);
  overflow: hidden;
  overflow-wrap: anywhere;
  color: rgb(var(--app-shell-text-primary));
  font-size: 0.875rem;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: normal;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.prompt-history-table__user {
  min-width: 180px;
  display: grid;
  gap: 2px;
}
</style>
