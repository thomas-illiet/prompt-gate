import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { FetchError } from 'ofetch'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  toProfileGroupsErrorMessage,
  useProfileGroups,
} from '../../app/composables/useProfileGroups'
import type { AccessGroup } from '../../app/types/groups'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return {
    apiFetch,
    useApiFetchMock: vi.fn(() => apiFetch),
  }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

function apiError(code: string) {
  return Object.assign(Object.create(FetchError.prototype), {
    response: {
      _data: { error: code },
    },
  }) as FetchError
}

const group: AccessGroup = {
  id: 'group-id',
  name: 'engineering',
  displayName: 'Engineering',
  description: 'Engineering model access',
  providers: [],
  modelPatterns: [],
  members: [],
  providerCount: 0,
  modelPatternCount: 0,
  memberCount: 1,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

describe('useProfileGroups', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
  })

  it('loads profile groups and supports reload', async () => {
    apiFetch.mockResolvedValueOnce([group]).mockResolvedValueOnce([])

    const profileGroups = useProfileGroups()
    await vi.waitFor(() => expect(profileGroups.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenNthCalledWith(1, '/api/v1/me/groups')
    expect(profileGroups.groups.value).toEqual([group])

    await profileGroups.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(2, '/api/v1/me/groups')
    expect(profileGroups.groups.value).toEqual([])
  })

  it('maps API errors to readable messages', async () => {
    apiFetch.mockRejectedValueOnce(apiError('groups service unavailable'))

    const profileGroups = useProfileGroups()
    await vi.waitFor(() => expect(profileGroups.loading.value).toBe(false))

    expect(profileGroups.groups.value).toEqual([])
    expect(profileGroups.error.value).toBe('groups service unavailable')
    expect(toProfileGroupsErrorMessage(new Error('boom'))).toBe('boom')
  })
})
