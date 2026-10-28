import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAdminNotifications } from '../../app/composables/useAdminNotifications'
import type {
  NotificationMessageListResponse,
  NotificationMessageSummary,
} from '../../app/types/notifications'

const { apiFetch, apiJson, useApiFetchMock, useApiJsonMock } = vi.hoisted(
  () => {
    const apiFetch = vi.fn()
    const apiJson = vi.fn()
    return {
      apiFetch,
      apiJson,
      useApiFetchMock: vi.fn(() => apiFetch),
      useApiJsonMock: vi.fn(() => apiJson),
    }
  },
)

mockNuxtImport('useApiFetch', () => useApiFetchMock)
mockNuxtImport('useApiJson', () => useApiJsonMock)

function summary(id: string): NotificationMessageSummary {
  return {
    id,
    kind: 'manual',
    subject: id,
    audience: 'users',
    recipients: 1,
    pending: 0,
    processing: 0,
    sent: 1,
    failed: 0,
    createdAt: '2026-10-28T00:00:00Z',
  }
}

function response(
  item: NotificationMessageSummary,
): NotificationMessageListResponse {
  return { items: [item], page: 1, pageSize: 20, total: 1 }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

describe('useAdminNotifications', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    apiJson.mockReset()
  })

  it('keeps the newest history response and aborts the superseded request', async () => {
    const first = deferred<NotificationMessageListResponse>()
    const second = deferred<NotificationMessageListResponse>()
    apiFetch
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise)

    const notifications = useAdminNotifications()
    const firstLoad = notifications.loadHistory()
    notifications.search.value = 'newest'
    const secondLoad = notifications.loadHistory()

    const firstSignal = apiFetch.mock.calls[0]?.[1]?.signal as AbortSignal
    expect(firstSignal.aborted).toBe(true)

    second.resolve(response(summary('newest')))
    await secondLoad
    first.resolve(response(summary('stale')))
    await firstLoad

    expect(notifications.items.value.map((item) => item.id)).toEqual(['newest'])
    expect(notifications.loading.value).toBe(false)
    expect(notifications.error.value).toBe('')
  })
})
