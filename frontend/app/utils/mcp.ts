import type { MCPHeaderPayload, MCPServerPayload } from '~/types/mcp'

export const MCP_SERVER_NAME_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/

export interface MCPHeaderFormRow {
  id: string
  name: string
  value: string
  sensitive: boolean
  hasValue: boolean
  clearValue: boolean
}

export interface MCPServerForm {
  name: string
  displayName: string
  url: string
  headers: MCPHeaderFormRow[]
  allowPattern: string
  denyPattern: string
  enabled: boolean
}

// normalizeMCPServerName canonicalizes MCP server names before validation.
export function normalizeMCPServerName(value: string) {
  return value.trim().toLowerCase()
}

// isValidMCPServerName checks the backend-compatible MCP server name format.
export function isValidMCPServerName(value: string) {
  return MCP_SERVER_NAME_PATTERN.test(normalizeMCPServerName(value))
}

// isValidMCPURL checks whether a URL can be used for an MCP server.
export function isValidMCPURL(value: string) {
  const url = value.trim()
  if (!url) {
    return false
  }

  try {
    const parsed = new URL(url)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

// normalizeMCPHeaderName trims a header name before validation and payloads.
export function normalizeMCPHeaderName(value: string) {
  return value.trim()
}

// isValidMCPHeaderName checks whether a header name is safe to send.
export function isValidMCPHeaderName(value: string) {
  const name = normalizeMCPHeaderName(value)
  return Boolean(name) && !/[ \t\r\n:]/.test(name)
}

// findDuplicateMCPHeaderNames returns case-insensitive duplicate header names.
export function findDuplicateMCPHeaderNames(rows: MCPHeaderFormRow[]) {
  const counts = new Map<string, number>()

  for (const row of rows) {
    const key = normalizeMCPHeaderName(row.name).toLowerCase()
    if (!key) {
      continue
    }
    counts.set(key, (counts.get(key) ?? 0) + 1)
  }

  return new Set(
    [...counts.entries()]
      .filter(([, count]) => count > 1)
      .map(([name]) => name),
  )
}

// buildMCPHeaderPayloads converts form header rows into API payloads.
export function buildMCPHeaderPayloads(rows: MCPHeaderFormRow[]) {
  return rows.map((row): MCPHeaderPayload => {
    const payload: MCPHeaderPayload = {
      name: normalizeMCPHeaderName(row.name),
      sensitive: row.sensitive,
    }
    const value = row.value.trim()

    if (!row.sensitive) {
      payload.value = value
      return payload
    }

    if (row.clearValue) {
      payload.value = null
      return payload
    }

    if (value) {
      payload.value = value
    }

    return payload
  })
}

// buildMCPServerPayload converts the MCP server form into an API payload.
export function buildMCPServerPayload(form: MCPServerForm): MCPServerPayload {
  return {
    name: normalizeMCPServerName(form.name),
    displayName: form.displayName.trim(),
    url: form.url.trim(),
    headers: buildMCPHeaderPayloads(form.headers),
    allowPattern: form.allowPattern.trim(),
    denyPattern: form.denyPattern.trim(),
    enabled: form.enabled,
  }
}
