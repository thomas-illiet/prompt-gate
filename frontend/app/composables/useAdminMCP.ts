import type {
  MCPServer,
  MCPServerListResponse,
  MCPServerPayload,
} from '~/types/mcp'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_header:
    'Header names must be unique and cannot contain spaces or colons.',
  invalid_name: 'Server name must use lowercase letters, numbers, and hyphens.',
  invalid_sort: 'Selected MCP server sort is invalid.',
  invalid_regex:
    'Tool allow or deny pattern must be a valid regular expression.',
  invalid_url: 'Server URL must be a valid HTTP or HTTPS URL.',
  mcp_server_not_found: 'MCP server no longer exists.',
  name_conflict: 'Another MCP server already uses this name.',
}

// toAdminMCPErrorMessage converts MCP API errors into user-facing text.
export function toAdminMCPErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected MCP server management error.',
  )
}

// useAdminMCP coordinates MCP server list state and mutations.
export function useAdminMCP() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()

  const saving = shallowRef(false)
  const selectedServer = shallowRef<MCPServer | null>(null)

  const queryList = useQueryList<MCPServer>({
    fetch: (queryString) =>
      apiFetch<MCPServerListResponse>(
        `/api/v1/admin/mcp/servers?${queryString}`,
      ),
    initialSortBy: 'name',
    initialSortDir: 'asc',
    toErrorMessage: toAdminMCPErrorMessage,
  })

  const enabledServersCount = computed(
    () => queryList.items.value.filter((server) => server.enabled).length,
  )

  // fetchServers refreshes MCP servers through the shared list composable.
  async function fetchServers() {
    await queryList.reload()
  }

  // reload exposes a stable refresh action for MCP views.
  async function reload() {
    await fetchServers()
  }

  // createServer stores a new MCP server and reloads the list.
  async function createServer(payload: MCPServerPayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'MCP server created.',
        toErrorMessage: toAdminMCPErrorMessage,
      },
      async () => {
        const server = await apiJson<MCPServer>(
          '/api/v1/admin/mcp/servers',
          payload,
          { method: 'POST' },
        )

        await fetchServers()
        return server
      },
    )
  }

  // loadServer fetches one MCP server for editing.
  async function loadServer(serverId: string) {
    selectedServer.value = await apiFetch<MCPServer>(
      `/api/v1/admin/mcp/servers/${serverId}`,
    )
    return selectedServer.value
  }

  // updateServer patches an MCP server and keeps the selected copy fresh.
  async function updateServer(serverId: string, payload: MCPServerPayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'MCP server updated.',
        toErrorMessage: toAdminMCPErrorMessage,
      },
      async () => {
        const server = await apiJson<MCPServer>(
          `/api/v1/admin/mcp/servers/${serverId}`,
          payload,
          { method: 'PATCH' },
        )

        selectedServer.value = server
        await fetchServers()
        return server
      },
    )
  }

  // deleteServer removes an MCP server and refreshes the list.
  async function deleteServer(serverId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'MCP server deleted.',
        toErrorMessage: toAdminMCPErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(`/api/v1/admin/mcp/servers/${serverId}`, {
          method: 'DELETE',
        })
        await fetchServers()
      },
    )
  }

  return {
    createServer,
    deleteServer,
    enabledServersCount,
    fetchServers,
    listError: queryList.listError,
    loadServer,
    loading: queryList.loading,
    page: queryList.page,
    pageSize: queryList.pageSize,
    reload,
    saving,
    selectedServer,
    servers: queryList.items,
    setPage: queryList.setPage,
    setPageSize: queryList.setPageSize,
    setSort: queryList.setSort,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    total: queryList.total,
    updateServer,
  }
}
