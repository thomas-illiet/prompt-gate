import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'
import { FetchError } from 'ofetch'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  toAdminGroupErrorMessage,
  useAdminGroups,
} from '../../app/composables/useAdminGroups'
import type {
  AccessGroup,
  GroupListResponse,
  GroupModelPatternValidationResponse,
} from '../../app/types/groups'

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
  description: 'Engineering access',
  providers: [],
  modelPatterns: ['^gpt-5'],
  members: [],
  providerCount: 0,
  modelPatternCount: 1,
  memberCount: 0,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function groupResponse(items: AccessGroup[]): GroupListResponse {
  return {
    items,
    page: 1,
    pageSize: 10,
    total: items.length,
  }
}

describe('useAdminGroups', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  it('loads groups on creation and supports reload', async () => {
    apiFetch
      .mockResolvedValueOnce(groupResponse([group]))
      .mockResolvedValueOnce(groupResponse([]))

    const adminGroups = useAdminGroups()
    await vi.waitFor(() => expect(adminGroups.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/admin/groups?page=1&pageSize=10&sortBy=name&sortDir=asc',
    )
    expect(adminGroups.groups.value).toEqual([group])

    await adminGroups.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/groups?page=1&pageSize=10&sortBy=name&sortDir=asc',
    )
    expect(adminGroups.groups.value).toEqual([])
  })

  it('creates a group and reloads the list', async () => {
    apiFetch
      .mockResolvedValueOnce(groupResponse([]))
      .mockResolvedValueOnce(group)
      .mockResolvedValueOnce(groupResponse([group]))

    const adminGroups = useAdminGroups()
    await vi.waitFor(() => expect(adminGroups.loading.value).toBe(false))

    await adminGroups.createGroup({
      name: 'engineering',
      displayName: 'Engineering',
      description: '',
      providerIds: [],
      modelPatterns: ['^gpt-5'],
    })

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/groups',
      expect.objectContaining({
        body: JSON.stringify({
          name: 'engineering',
          displayName: 'Engineering',
          description: '',
          providerIds: [],
          modelPatterns: ['^gpt-5'],
        }),
        method: 'POST',
      }),
    )
    expect(adminGroups.groups.value).toEqual([group])
  })

  it('loads provider and member options across all pages', async () => {
    apiFetch
      .mockResolvedValueOnce(groupResponse([]))
      .mockResolvedValueOnce({
        items: [
          {
            id: 'provider-a',
            name: 'alpha',
            displayName: 'Alpha',
            type: 'openai',
            enabled: true,
          },
        ],
        total: 2,
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 'provider-b',
            name: 'beta',
            displayName: 'Beta',
            type: 'anthropic',
            enabled: true,
          },
        ],
        total: 2,
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 'user-id',
            preferredUsername: 'ada',
            email: 'ada@example.com',
            name: 'Ada',
            role: 'user',
            isActive: true,
          },
        ],
        total: 1,
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 'service-id',
            identifier: 'service',
            name: 'Service',
            role: 'user',
            isActive: true,
          },
        ],
        total: 1,
      })

    const adminGroups = useAdminGroups()
    await vi.waitFor(() => expect(adminGroups.loading.value).toBe(false))

    await adminGroups.loadProviderOptions()
    await adminGroups.loadMemberOptions()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/providers?page=1&pageSize=100&sortBy=name&sortDir=asc',
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      '/api/v1/admin/providers?page=2&pageSize=100&sortBy=name&sortDir=asc',
    )
    expect(
      adminGroups.providerOptions.value.map((provider) => provider.id),
    ).toEqual(['provider-a', 'provider-b'])
    expect(adminGroups.memberOptions.value.map((member) => member.id)).toEqual([
      'user-id',
      'service-id',
    ])
  })

  it('validates model regex against provider models', async () => {
    const validation: GroupModelPatternValidationResponse = {
      matchedModelCount: 2,
      matchedModels: ['gpt-5-mini', 'gpt-5.1-codex'],
      providerResults: [
        {
          id: 'provider-id',
          name: 'openai-main',
          displayName: 'OpenAI Main',
          availableModelCount: 3,
          matchedModelCount: 2,
          matchedModels: ['gpt-5-mini', 'gpt-5.1-codex'],
        },
      ],
      unavailableProviderCount: 0,
    }
    apiFetch
      .mockResolvedValueOnce(groupResponse([]))
      .mockResolvedValueOnce(validation)

    const adminGroups = useAdminGroups()
    await vi.waitFor(() => expect(adminGroups.loading.value).toBe(false))

    await adminGroups.validateModelPatterns({
      providerIds: ['provider-id'],
      modelPatterns: ['^gpt-5'],
    })

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/groups/model-patterns/validate',
      expect.objectContaining({
        body: JSON.stringify({
          providerIds: ['provider-id'],
          modelPatterns: ['^gpt-5'],
        }),
        method: 'POST',
      }),
    )
    expect(adminGroups.modelValidation.value).toEqual(validation)

    adminGroups.clearModelValidation()

    expect(adminGroups.modelValidation.value).toBeNull()
  })

  it('maps group API errors to readable messages', () => {
    expect(toAdminGroupErrorMessage(apiError('invalid_regex'))).toBe(
      'One or more model patterns are invalid regular expressions.',
    )
  })
})
