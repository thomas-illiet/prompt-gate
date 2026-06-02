export type ProviderType = 'openai' | 'anthropic' | 'ollama'

export interface Provider {
  id: string
  name: string
  displayName: string
  type: ProviderType
  baseUrl: string
  hasApiKey: boolean
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface ProviderListResponse {
  items: Provider[]
  page: number
  pageSize: number
  total: number
}

export interface UpdateProviderPayload {
  displayName: string
  type: ProviderType
  baseUrl: string
  apiKey?: string
  enabled: boolean
}

export interface CreateProviderPayload extends UpdateProviderPayload {
  name: string
}
