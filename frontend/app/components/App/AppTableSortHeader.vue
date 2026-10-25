<script setup lang="ts">
import type { AppTableSortDir } from '~/utils/table'

const props = withDefaults(
  defineProps<{
    active: boolean
    align?: 'center' | 'start'
    direction: AppTableSortDir
    label: string
  }>(),
  {
    align: 'start',
  },
)

const emit = defineEmits<{
  sort: []
}>()

const ariaSort = computed(() => {
  if (!props.active) return undefined
  return props.direction === 'asc' ? 'ascending' : 'descending'
})

const icon = computed(() => {
  if (!props.active) return 'mdi-unfold-more-vertical'
  return props.direction === 'asc' ? 'mdi-arrow-up' : 'mdi-arrow-down'
})
</script>

<template>
  <th
    class="app-table-sort-header"
    :class="{
      'app-table-sort-header--center': props.align === 'center',
    }"
    scope="col"
    :aria-sort="ariaSort"
  >
    <button
      class="app-table-sort-header__button"
      type="button"
      :aria-label="`Sort by ${props.label}`"
      @click="emit('sort')"
    >
      <span>{{ props.label }}</span>
      <v-icon :icon="icon" size="16" aria-hidden="true" />
    </button>
  </th>
</template>

<style scoped>
.app-table-sort-header--center {
  text-align: center !important;
}

.app-table-sort-header__button {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 4px;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: inherit;
  font-weight: inherit;
  letter-spacing: inherit;
  text-transform: inherit;
}

.app-table-sort-header--center .app-table-sort-header__button {
  justify-content: center;
}

.app-table-sort-header__button:hover {
  color: rgb(var(--v-theme-primary));
}
</style>
