export type MCPHeaderValue = string | null | undefined

export interface MCPHeader {
  name: string
  value?: string
  sensitive: boolean
  hasValue: boolean
}

export interface MCPServer {
  id: string
  name: string
  displayName: string
  url: string
  headers: MCPHeader[]
  allowPattern: string
  denyPattern: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface MCPServerListResponse {
  items: MCPServer[]
  page: number
  pageSize: number
  total: number
}

export interface MCPHeaderPayload {
  name: string
  value?: MCPHeaderValue
  sensitive: boolean
}

export interface MCPServerPayload {
  name: string
  displayName: string
  url: string
  headers: MCPHeaderPayload[]
  allowPattern: string
  denyPattern: string
  enabled: boolean
}
