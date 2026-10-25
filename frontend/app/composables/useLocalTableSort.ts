import { computed, shallowRef, toRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'

import type { AppTableSortDir } from '~/utils/table'

export type AppTableSortValue = number | string | null | undefined

interface UseLocalTableSortOptions<TItem, TSortKey extends string> {
  firstSortDir?: (key: TSortKey) => AppTableSortDir
  getValue: (item: TItem, key: TSortKey) => AppTableSortValue
  initialSortBy: MaybeRefOrGetter<TSortKey>
  initialSortDir?: MaybeRefOrGetter<AppTableSortDir>
}

const tableCollator = new Intl.Collator('en', {
  numeric: true,
  sensitivity: 'base',
})

// useLocalTableSort provides stable, accessible sorting for small client-side tables.
export function useLocalTableSort<TItem, TSortKey extends string>(
  items: MaybeRefOrGetter<readonly TItem[]>,
  options: UseLocalTableSortOptions<TItem, TSortKey>,
) {
  const initialSortDir = options.initialSortDir ?? 'asc'
  const sortBy = shallowRef<TSortKey>(toValue(options.initialSortBy))
  const sortDir = shallowRef<AppTableSortDir>(toValue(initialSortDir))

  watch(
    [toRef(options.initialSortBy), toRef(initialSortDir)],
    ([nextSortBy, nextSortDir]) => {
      sortBy.value = nextSortBy
      sortDir.value = nextSortDir
    },
  )

  const sortedItems = computed(() =>
    toValue(items)
      .map((item, index) => ({ index, item }))
      .sort((left, right) => {
        const leftValue = options.getValue(left.item, sortBy.value)
        const rightValue = options.getValue(right.item, sortBy.value)
        if (leftValue == null && rightValue == null)
          return left.index - right.index
        if (leftValue == null) return 1
        if (rightValue == null) return -1

        const comparison = compareTableValues(leftValue, rightValue)
        if (comparison === 0) return left.index - right.index
        return sortDir.value === 'asc' ? comparison : -comparison
      })
      .map(({ item }) => item),
  )

  function toggleSort(key: TSortKey) {
    if (sortBy.value === key) {
      sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
      return
    }

    sortBy.value = key
    sortDir.value = options.firstSortDir?.(key) ?? 'asc'
  }

  function isSortedBy(key: TSortKey) {
    return sortBy.value === key
  }

  return {
    isSortedBy,
    sortedItems,
    sortBy,
    sortDir,
    toggleSort,
  }
}

function compareTableValues(
  left: Exclude<AppTableSortValue, null | undefined>,
  right: Exclude<AppTableSortValue, null | undefined>,
) {
  if (typeof left === 'number' && typeof right === 'number') {
    return left - right
  }
  return tableCollator.compare(String(left), String(right))
}
