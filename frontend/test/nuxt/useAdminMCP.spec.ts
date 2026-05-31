import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import { useAdminMCP, toAdminMCPErrorMessage } from '../../app/composables/useAdminMCP'
import type {
  MCPServer,
  MCPServerListResponse,
  MCPServerPayload,
} from '../../app/types/mcp'

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

const server: MCPServer = {
  id: 'server-id',
  name: 'linear-tools',
  displayName: 'Linear tools',
  url: 'https://mcp.example.com/mcp',
  headers: [
    {
      name: 'Authorization',
      sensitive: true,
      hasValue: true,
    },
  ],
  allowPattern: '^linear_',
  denyPattern: '',
  enabled: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const payload: MCPServerPayload = {
  name: 'linear-tools',
  displayName: 'Linear tools',
  url: 'https://mcp.example.com/mcp',
  headers: [{ name: 'Authorization', sensitive: true }],
  allowPattern: '^linear_',
  denyPattern: '',
  enabled: true,
}

function response(
  items: MCPServer[],
  total = items.length,
): MCPServerListResponse {
  return {
    items,
    page: 1,
    pageSize: 10,
    total,
  }
}

describe('useAdminMCP', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  it('loads servers on creation and supports reload', async () => {
    apiFetch
      .mockResolvedValueOnce(response([server]))
      .mockResolvedValueOnce(response([]))

    const adminMCP = useAdminMCP()
    await vi.waitFor(() => expect(adminMCP.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/admin/mcp/servers?page=1&pageSize=10&sortBy=name&sortDir=asc',
    )
    expect(adminMCP.servers.value).toEqual([server])
    expect(adminMCP.enabledServersCount.value).toBe(1)

    await adminMCP.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/mcp/servers?page=1&pageSize=10&sortBy=name&sortDir=asc',
    )
    expect(adminMCP.servers.value).toEqual([])
  })

  it('stores list errors without throwing', async () => {
    apiFetch.mockRejectedValueOnce(apiError('invalid_url'))

    const adminMCP = useAdminMCP()
    await vi.waitFor(() => expect(adminMCP.loading.value).toBe(false))

    expect(adminMCP.listError.value).toBe(
      'Server URL must be a valid HTTP or HTTPS URL.',
    )
  })

  it('creates, updates, loads, and deletes servers through admin endpoints', async () => {
    apiFetch
      .mockResolvedValueOnce(response([]))
      .mockResolvedValueOnce(server)
      .mockResolvedValueOnce(response([server]))
      .mockResolvedValueOnce(server)
      .mockResolvedValueOnce(server)
      .mockResolvedValueOnce(response([server]))
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(response([]))

    const adminMCP = useAdminMCP()
    await vi.waitFor(() => expect(adminMCP.loading.value).toBe(false))

    await adminMCP.createServer(payload)
    await adminMCP.loadServer(server.id)
    await adminMCP.updateServer(server.id, payload)
    await adminMCP.deleteServer(server.id)

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/mcp/servers',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      4,
      `/api/v1/admin/mcp/servers/${server.id}`,
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      5,
      `/api/v1/admin/mcp/servers/${server.id}`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      7,
      `/api/v1/admin/mcp/servers/${server.id}`,
      { method: 'DELETE' },
    )
  })

  it('maps API errors to readable messages', () => {
    expect(toAdminMCPErrorMessage(apiError('mcp_server_not_found'))).toBe(
      'MCP server no longer exists.',
    )
    expect(toAdminMCPErrorMessage(apiError('name_conflict'))).toBe(
      'Another MCP server already uses this name.',
    )
    expect(toAdminMCPErrorMessage(apiError('invalid_header'))).toBe(
      'Header names must be unique and cannot contain spaces or colons.',
    )
    expect(toAdminMCPErrorMessage(apiError('invalid_regex'))).toBe(
      'Tool allow or deny pattern must be a valid regular expression.',
    )
  })
})
