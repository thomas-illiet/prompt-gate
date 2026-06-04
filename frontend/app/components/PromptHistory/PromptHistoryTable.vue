<script setup lang="ts">
import { useLocalStorage } from '@vueuse/core'
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
type PromptHistoryColumnKey =
  | 'actions'
  | 'clientIp'
  | 'createdAt'
  | 'durationMs'
  | 'inputTokens'
  | 'model'
  | 'outputTokens'
  | 'prompt'
  | 'provider'
  | 'totalTokens'
  | 'userName'

interface PromptHistoryColumnDefinition {
  adminOnly?: boolean
  defaultVisible: boolean
  header: DataTableHeader
  key: PromptHistoryColumnKey
  required?: boolean
}

const PROMPT_PREVIEW_MAX_LENGTH = 220
const DEFAULT_COLUMN_PREFERENCES_KEY = 'promptgate.promptHistory.columns.v1'
const REQUIRED_COLUMN_KEYS = new Set<PromptHistoryColumnKey>([
  'prompt',
  'createdAt',
  'actions',
])

const props = withDefaults(
  defineProps<{
    columnPreferencesKey?: string
    enableColumnPicker?: boolean
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
    columnPreferencesKey: DEFAULT_COLUMN_PREFERENCES_KEY,
    enableColumnPicker: false,
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

const columnDefinitions = computed<PromptHistoryColumnDefinition[]>(() => [
  {
    defaultVisible: true,
    header: { title: 'Prompt', key: 'prompt', width: 360 },
    key: 'prompt',
    required: true,
  },
  {
    adminOnly: true,
    defaultVisible: true,
    header: { title: 'User', key: 'userName' },
    key: 'userName',
  },
  {
    defaultVisible: true,
    header: { title: 'Provider', key: 'provider' },
    key: 'provider',
  },
  {
    defaultVisible: true,
    header: { title: 'Model', key: 'model' },
    key: 'model',
  },
  {
    defaultVisible: false,
    header: appTableCenteredColumn({
      title: 'Input tokens',
      key: 'inputTokens',
    }),
    key: 'inputTokens',
  },
  {
    defaultVisible: false,
    header: appTableCenteredColumn({
      title: 'Output tokens',
      key: 'outputTokens',
    }),
    key: 'outputTokens',
  },
  {
    defaultVisible: false,
    header: appTableCenteredColumn({
      title: 'Total tokens',
      key: 'totalTokens',
    }),
    key: 'totalTokens',
  },
  {
    defaultVisible: false,
    header: appTableCenteredColumn({
      title: 'Duration',
      key: 'durationMs',
    }),
    key: 'durationMs',
  },
  {
    adminOnly: true,
    defaultVisible: false,
    header: { title: 'Client IP', key: 'clientIp' },
    key: 'clientIp',
  },
  {
    defaultVisible: true,
    header: appTableCenteredColumn({
      title: 'Created',
      key: 'createdAt',
    }),
    key: 'createdAt',
    required: true,
  },
  {
    defaultVisible: true,
    header: appTableCenteredColumn({
      title: 'Actions',
      key: 'actions',
      sortable: false,
    }),
    key: 'actions',
    required: true,
  },
])

const availableColumnDefinitions = computed(() =>
  columnDefinitions.value.filter(
    (definition) => !definition.adminOnly || props.showUser,
  ),
)

const availableColumnKeys = computed(
  () =>
    new Set(
      availableColumnDefinitions.value.map((definition) => definition.key),
    ),
)

const defaultVisibleColumnKeys = computed(() =>
  availableColumnDefinitions.value
    .filter((definition) => definition.defaultVisible || definition.required)
    .map((definition) => definition.key),
)

const persistedVisibleColumnKeys = useLocalStorage<PromptHistoryColumnKey[]>(
  props.columnPreferencesKey,
  defaultVisibleColumnKeys.value,
)

const visibleColumnKeys = computed(() => {
  if (!props.enableColumnPicker) {
    return availableColumnDefinitions.value.map((definition) => definition.key)
  }

  return normalizeVisibleColumnKeys(persistedVisibleColumnKeys.value)
})

const headers = computed<DataTableHeader[]>(() => {
  const visible = new Set(visibleColumnKeys.value)
  return availableColumnDefinitions.value
    .filter((definition) => visible.has(definition.key))
    .map((definition) => definition.header)
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

// normalizeVisibleColumnKeys keeps persisted preferences compatible with the current scope.
function normalizeVisibleColumnKeys(keys: readonly PromptHistoryColumnKey[]) {
  const normalized = new Set<PromptHistoryColumnKey>()
  for (const key of keys) {
    if (availableColumnKeys.value.has(key)) {
      normalized.add(key)
    }
  }

  for (const key of REQUIRED_COLUMN_KEYS) {
    if (availableColumnKeys.value.has(key)) {
      normalized.add(key)
    }
  }

  if (normalized.size === 0) {
    return defaultVisibleColumnKeys.value
  }

  return availableColumnDefinitions.value
    .map((definition) => definition.key)
    .filter((key) => normalized.has(key))
}

// isColumnVisible reports whether the column is currently rendered.
function isColumnVisible(key: PromptHistoryColumnKey) {
  return visibleColumnKeys.value.includes(key)
}

// toggleColumn updates persisted visibility while preserving required columns.
function toggleColumn(key: PromptHistoryColumnKey, visible: boolean) {
  if (REQUIRED_COLUMN_KEYS.has(key)) {
    return
  }

  const next = new Set(visibleColumnKeys.value)
  if (visible) {
    next.add(key)
  } else {
    next.delete(key)
  }

  persistedVisibleColumnKeys.value = availableColumnDefinitions.value
    .map((definition) => definition.key)
    .filter((definitionKey) => next.has(definitionKey))

  resetSortIfHidden(key)
}

// resetColumns restores the scope-specific default column set.
function resetColumns() {
  const previouslySortedColumn = props.sortBy as PromptHistoryColumnKey
  persistedVisibleColumnKeys.value = defaultVisibleColumnKeys.value
  resetSortIfHidden(previouslySortedColumn)
}

// resetSortIfHidden avoids keeping the table sorted by a hidden column.
function resetSortIfHidden(key: PromptHistoryColumnKey) {
  if (props.sortBy === key && !visibleColumnKeys.value.includes(key)) {
    emit('update:sort', 'createdAt', 'desc')
  }
}

// userLabel returns the best available display identity for admin prompt rows.
function userLabel(item: PromptHistoryTableItem) {
  if (!('userId' in item)) {
    return ''
  }

  return item.userName || item.userPreferredUsername || item.userId
}

// clientIpLabel returns the client IP for admin prompt rows.
function clientIpLabel(item: PromptHistoryTableItem) {
  if (!('clientIp' in item)) {
    return ''
  }

  return item.clientIp || 'Unknown'
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
      <v-menu
        v-if="props.enableColumnPicker"
        location="bottom end"
        :close-on-content-click="false"
        offset="8"
      >
        <template #activator="{ props: menuProps }">
          <v-btn
            v-bind="menuProps"
            color="primary"
            variant="tonal"
            rounded="xl"
            prepend-icon="mdi-table-column"
          >
            Columns
          </v-btn>
        </template>

        <v-card class="prompt-history-table__columns-menu" min-width="240">
          <v-list density="compact">
            <v-list-item
              v-for="definition in availableColumnDefinitions"
              :key="definition.key"
              class="prompt-history-table__column-item"
            >
              <v-checkbox
                :model-value="isColumnVisible(definition.key)"
                :disabled="definition.required"
                :label="String(definition.header.title)"
                color="primary"
                density="compact"
                hide-details
                @update:model-value="
                  toggleColumn(definition.key, Boolean($event))
                "
              />
            </v-list-item>
          </v-list>

          <v-divider />

          <v-card-actions>
            <v-spacer />
            <v-btn
              color="primary"
              variant="text"
              rounded="lg"
              @click="resetColumns"
            >
              Reset
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-menu>

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

      <template #item.totalTokens="{ item }">
        <span class="app-table-text">
          {{ formatNumber(item.totalTokens) }}
        </span>
      </template>

      <template #item.durationMs="{ item }">
        <span class="app-table-text">
          {{ formatDurationMs(item.durationMs) }}
        </span>
      </template>

      <template #item.clientIp="{ item }">
        <span class="app-table-text">
          {{ clientIpLabel(item) }}
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

.prompt-history-table__columns-menu {
  max-height: min(520px, calc(100vh - 120px));
}

.prompt-history-table__column-item {
  min-height: 38px;
}

.prompt-history-table__column-item :deep(.v-selection-control) {
  min-height: 34px;
}
</style>
