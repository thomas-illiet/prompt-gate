import { nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useQueryList } from '../../app/composables/useQueryList'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve
  })
  return { promise, resolve }
}

describe('useQueryList', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('debounces search updates and resets to the first page', async () => {
    vi.useFakeTimers()
    const fetch = vi
      .fn()
      .mockResolvedValue({ items: [] as string[], total: 0 })

    const list = useQueryList<string>({
      debounceMs: 50,
      fetch,
      toErrorMessage: () => 'List failed.',
    })

    await vi.waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        'page=1&pageSize=10&sortBy=createdAt&sortDir=desc',
      ),
    )

    list.setPage(3)
    await nextTick()
    expect(fetch).toHaveBeenLastCalledWith(
      'page=3&pageSize=10&sortBy=createdAt&sortDir=desc',
    )

    list.setSearch('alpha')
    expect(list.page.value).toBe(1)
    await vi.advanceTimersByTimeAsync(50)

    expect(fetch).toHaveBeenLastCalledWith(
      'page=1&pageSize=10&sortBy=createdAt&sortDir=desc&search=alpha',
    )

    list.setSort('name', 'asc')
    await nextTick()
    expect(list.page.value).toBe(1)
    expect(fetch).toHaveBeenLastCalledWith(
      'page=1&pageSize=10&sortBy=name&sortDir=asc&search=alpha',
    )
  })

  it('ignores stale responses when a newer request has completed', async () => {
    const first = deferred<{ items: string[]; total: number }>()
    const second = deferred<{ items: string[]; total: number }>()
    const fetch = vi
      .fn()
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)

    const list = useQueryList<string>({
      debounceMs: 0,
      fetch,
      toErrorMessage: () => 'List failed.',
    })

    expect(fetch).toHaveBeenCalledWith(
      'page=1&pageSize=10&sortBy=createdAt&sortDir=desc',
    )
    list.setPage(2)
    await nextTick()
    expect(fetch).toHaveBeenCalledWith(
      'page=2&pageSize=10&sortBy=createdAt&sortDir=desc',
    )

    second.resolve({ items: ['new'], total: 1 })
    await nextTick()
    await vi.waitFor(() => expect(list.items.value).toEqual(['new']))

    first.resolve({ items: ['stale'], total: 1 })
    await nextTick()

    expect(list.items.value).toEqual(['new'])
  })

  it('keeps reload loading visible for the configured minimum duration', async () => {
    vi.useFakeTimers()
    const reloadResponse = deferred<{ items: string[]; total: number }>()
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({ items: ['initial'], total: 1 })
      .mockReturnValueOnce(reloadResponse.promise)

    const list = useQueryList<string>({
      debounceMs: 0,
      fetch,
      reloadMinLoadingMs: 500,
      toErrorMessage: () => 'List failed.',
    })

    await vi.waitFor(() => expect(list.loading.value).toBe(false))

    const reloadPromise = list.reload()
    expect(list.loading.value).toBe(true)

    reloadResponse.resolve({ items: ['refreshed'], total: 1 })
    await Promise.resolve()
    await Promise.resolve()
    await nextTick()

    expect(list.items.value).toEqual(['refreshed'])

    await vi.advanceTimersByTimeAsync(499)
    expect(list.loading.value).toBe(true)

    await vi.advanceTimersByTimeAsync(1)
    await reloadPromise

    expect(list.loading.value).toBe(false)
  })
})
