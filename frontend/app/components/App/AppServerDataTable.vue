<script setup lang="ts" generic="TItem extends Record<string, any>">
import { computed, onScopeDispose, shallowRef, useSlots, watch } from 'vue'
import type { DataTableHeader } from 'vuetify'

import type { AppTableSortDir } from '~/utils/table'
import { appTableItemsPerPageOptions } from '~/utils/table'

type AppTableEmptyTone = 'primary' | 'success' | 'warning' | 'error' | 'info'

const props = withDefaults(
  defineProps<{
    defaultSortBy: string
    defaultSortDir?: AppTableSortDir
    emptyIcon?: string
    emptyText?: string
    emptyTitle?: string
    emptyTone?: AppTableEmptyTone
    headers: DataTableHeader[]
    hover?: boolean
    itemValue?: string
    items: TItem[]
    itemsPerPageOptions?: readonly number[]
    loading: boolean
    loadingDelayMs?: number
    mustSort?: boolean
    page: number
    pageSize: number
    refreshIndicatorMinMs?: number
    sortBy: string
    sortDir: AppTableSortDir
    total: number
  }>(),
  {
    defaultSortDir: 'desc',
    emptyIcon: 'mdi-table-off',
    emptyText: 'There is nothing to show for the current filters.',
    emptyTitle: 'No results',
    emptyTone: 'primary',
    hover: true,
    itemValue: 'id',
    itemsPerPageOptions: () => [...appTableItemsPerPageOptions],
    loadingDelayMs: 180,
    mustSort: true,
    refreshIndicatorMinMs: 320,
  },
)

const slots = useSlots()

const emit = defineEmits<{
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: AppTableSortDir]
}>()

const tableSortBy = computed(() => [
  { key: props.sortBy, order: props.sortDir },
])

const forwardedSlotNames = computed(() =>
  Object.keys(slots).filter(
    (slotName) => slotName !== 'loading' && slotName !== 'no-data',
  ),
)

const delayedLoading = shallowRef(false)
const visibleRefresh = shallowRef(false)
let loadingDelayTimer: ReturnType<typeof setTimeout> | null = null
let refreshIndicatorTimer: ReturnType<typeof setTimeout> | null = null
let refreshStartedAt = 0

const tableLoading = computed(
  () => props.loading && (delayedLoading.value || props.items.length === 0),
)

const loadingSkeletonRows = computed(() =>
  Array.from(
    { length: Math.min(Math.max(props.pageSize, 1), 10) },
    (_, index) => index,
  ),
)

const loadingSkeletonCells = computed(() =>
  Array.from({ length: Math.max(props.headers.length, 1) }, (_, index) => index),
)

const loadingSkeletonGridTemplate = computed(
  () => `repeat(${loadingSkeletonCells.value.length}, minmax(72px, 1fr))`,
)

function clearLoadingDelayTimer() {
  if (loadingDelayTimer) {
    clearTimeout(loadingDelayTimer)
    loadingDelayTimer = null
  }
}

function clearRefreshIndicatorTimer() {
  if (refreshIndicatorTimer) {
    clearTimeout(refreshIndicatorTimer)
    refreshIndicatorTimer = null
  }
}

watch(
  () =>
    [
      props.loading,
      props.loadingDelayMs,
      props.refreshIndicatorMinMs,
    ] as const,
  ([loading, loadingDelayMs, refreshIndicatorMinMs]) => {
    clearLoadingDelayTimer()
    clearRefreshIndicatorTimer()

    if (!loading) {
      delayedLoading.value = false
      if (!visibleRefresh.value) {
        return
      }

      const remainingMs = Math.max(
        refreshIndicatorMinMs - (Date.now() - refreshStartedAt),
        0,
      )
      if (remainingMs === 0) {
        visibleRefresh.value = false
        return
      }

      refreshIndicatorTimer = setTimeout(() => {
        visibleRefresh.value = false
        refreshIndicatorTimer = null
      }, remainingMs)
      return
    }

    refreshStartedAt = Date.now()
    visibleRefresh.value = true

    if (loadingDelayMs <= 0) {
      delayedLoading.value = true
      return
    }

    loadingDelayTimer = setTimeout(() => {
      delayedLoading.value = true
      loadingDelayTimer = null
    }, loadingDelayMs)
  },
  { immediate: true },
)

onScopeDispose(() => {
  clearLoadingDelayTimer()
  clearRefreshIndicatorTimer()
})

// updateSort emits normalized sort changes from Vuetify's table model.
function updateSort(value: readonly { key: string; order?: boolean | string }[]) {
  const next = value[0]
  if (!next?.key) {
    emit('update:sort', props.defaultSortBy, props.defaultSortDir)
    return
  }

  emit('update:sort', next.key, next.order === 'asc' ? 'asc' : 'desc')
}
</script>

<template>
  <div
    class="app-server-data-table-shell"
    :class="{ 'app-server-data-table-shell--loading': visibleRefresh }"
  >
    <span
      v-if="visibleRefresh"
      class="app-server-data-table-shell__refresh"
      data-test="table-refresh-indicator"
      aria-hidden="true"
    />

    <v-data-table-server
      class="app-server-data-table"
      :aria-busy="props.loading"
      :headers="props.headers"
      :hover="props.hover"
      :items="props.items"
      :items-length="props.total"
      :items-per-page="props.pageSize"
      :items-per-page-options="props.itemsPerPageOptions"
      :loading="tableLoading"
      :must-sort="props.mustSort"
      :page="props.page"
      :sort-by="tableSortBy"
      :item-value="props.itemValue"
      @update:items-per-page="emit('update:page-size', $event)"
      @update:page="emit('update:page', $event)"
      @update:sort-by="updateSort"
    >
      <template #loading>
        <slot v-if="$slots.loading" name="loading" />
        <div
          v-else
          class="app-server-data-table__skeleton"
          data-test="table-loading-skeleton"
          aria-hidden="true"
        >
          <div
            v-for="rowIndex in loadingSkeletonRows"
            :key="rowIndex"
            class="app-server-data-table__skeleton-row"
            :style="{ gridTemplateColumns: loadingSkeletonGridTemplate }"
          >
            <span
              v-for="cellIndex in loadingSkeletonCells"
              :key="cellIndex"
              class="app-server-data-table__skeleton-cell"
              :class="{
                'app-server-data-table__skeleton-cell--medium':
                  cellIndex % 3 === 1,
                'app-server-data-table__skeleton-cell--short':
                  cellIndex === loadingSkeletonCells.length - 1,
              }"
            />
          </div>
        </div>
      </template>

      <template #no-data>
        <slot v-if="$slots['no-data']" name="no-data" />
        <AppEmptyState
          v-else
          compact
          :icon="props.emptyIcon"
          :title="props.emptyTitle"
          :text="props.emptyText"
          :tone="props.emptyTone"
        />
      </template>

      <template v-for="slotName in forwardedSlotNames" #[slotName]="slotProps">
        <slot :name="slotName" v-bind="slotProps" />
      </template>
    </v-data-table-server>
  </div>
</template>

<style scoped>
.app-server-data-table-shell {
  position: relative;
  overflow: hidden;
}

.app-server-data-table-shell__refresh {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  z-index: 2;
  height: 3px;
  overflow: hidden;
  pointer-events: none;
}

.app-server-data-table-shell__refresh::before {
  position: absolute;
  inset-block: 0;
  left: 0;
  width: 42%;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(var(--v-theme-primary), 0.72) 45%,
    transparent 100%
  );
  animation: app-server-data-table-refresh 1s ease-in-out infinite;
  content: '';
}

.app-server-data-table-shell--loading :deep(.v-table__wrapper) {
  opacity: 0.96;
  transition: opacity 120ms ease;
}

.app-server-data-table__skeleton {
  display: grid;
  gap: 10px;
  padding: 8px 0;
}

.app-server-data-table__skeleton-row {
  display: grid;
  min-height: 42px;
  align-items: center;
  gap: 16px;
}

.app-server-data-table__skeleton-cell {
  display: block;
  width: 100%;
  max-width: 220px;
  height: 14px;
  border-radius: 999px;
  background:
    linear-gradient(
      90deg,
      rgba(var(--app-shell-border), 0.34) 0%,
      rgba(var(--v-theme-primary), 0.14) 45%,
      rgba(var(--app-shell-border), 0.34) 90%
    );
  background-size: 220% 100%;
  animation: app-server-data-table-skeleton 1.4s ease-in-out infinite;
}

.app-server-data-table__skeleton-cell--medium {
  max-width: 160px;
}

.app-server-data-table__skeleton-cell--short {
  max-width: 92px;
}

@keyframes app-server-data-table-refresh {
  0% {
    transform: translateX(-110%);
  }

  100% {
    transform: translateX(240%);
  }
}

@keyframes app-server-data-table-skeleton {
  0% {
    background-position: 120% 0;
  }

  100% {
    background-position: -120% 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-server-data-table-shell__refresh::before,
  .app-server-data-table__skeleton-cell {
    animation: none;
  }
}
</style>
