import { nextTick, shallowRef } from 'vue'
import { describe, expect, it } from 'vitest'

import { useLocalTableSort } from '../../app/composables/useLocalTableSort'

type SortKey = 'name' | 'score'

describe('useLocalTableSort', () => {
  it('accepts reactive inputs, stays stable, and keeps missing values last', async () => {
    const items = shallowRef([
      { id: 'missing', name: 'No score', score: null as number | null },
      { id: 'high', name: 'Zulu', score: 20 },
      { id: 'low-a', name: 'Alpha', score: 10 },
      { id: 'low-b', name: 'Beta', score: 10 },
    ])
    const initialSortBy = shallowRef<SortKey>('name')
    const sorting = useLocalTableSort(items, {
      firstSortDir: (key) => (key === 'score' ? 'desc' : 'asc'),
      getValue: (item, key) => item[key],
      initialSortBy,
      initialSortDir: 'asc',
    })

    expect(sorting.sortedItems.value.map(({ id }) => id)).toEqual([
      'low-a',
      'low-b',
      'missing',
      'high',
    ])

    sorting.toggleSort('score')
    expect(sorting.sortDir.value).toBe('desc')
    expect(sorting.sortedItems.value.map(({ id }) => id)).toEqual([
      'high',
      'low-a',
      'low-b',
      'missing',
    ])

    sorting.toggleSort('score')
    expect(sorting.sortedItems.value.map(({ id }) => id)).toEqual([
      'low-a',
      'low-b',
      'high',
      'missing',
    ])

    initialSortBy.value = 'score'
    await nextTick()
    expect(sorting.sortBy.value).toBe('score')

    initialSortBy.value = 'name'
    items.value = [
      { id: 'charlie', name: 'Charlie', score: 1 },
      { id: 'bravo', name: 'Bravo', score: 2 },
    ]
    await nextTick()

    expect(sorting.sortDir.value).toBe('asc')
    expect(sorting.sortedItems.value.map(({ id }) => id)).toEqual([
      'bravo',
      'charlie',
    ])
  })
})
