export type ModelLabProviderType = 'anthropic' | 'ollama' | 'openai'
export type ModelLabProtocol = 'anthropic-messages' | 'openai-chat'
export type ModelLabCatalogState = 'fresh' | 'stale' | 'unavailable'
export type ModelLabPriceSource = 'exact' | 'fallback' | 'unavailable'
export type ModelLabAvailabilityState =
  | 'available'
  | 'degraded'
  | 'unavailable'
  | 'unknown'

export interface ModelLabPricing {
  currency: string
  input: number | null
  output: number | null
  source: ModelLabPriceSource
  unit: string
}

export interface ModelLabObservedAvailability {
  durationP50Ms: number | null
  lastObservedAt: string | null
  sampleSize: number
  state: ModelLabAvailabilityState
  successRate: number | null
  ttftP50Ms: number | null
  windowSeconds: number
}

export interface ModelLabModel {
  displayName: string | null
  id: string
  observedAvailability?: ModelLabObservedAvailability | null
  playground: {
    protocol: ModelLabProtocol
    supported: boolean
  }
  pricing: ModelLabPricing | null
}

export interface ModelLabProvider {
  catalog: {
    fetchedAt: string | null
    state: ModelLabCatalogState
  }
  displayName: string
  models: ModelLabModel[]
  name: string
  type: ModelLabProviderType
}

export interface ModelLabCatalogResponse {
  enabled?: boolean
  generatedAt: string
  limits: ModelLabLimits
  providers: ModelLabProvider[]
}

export interface ModelLabLimits {
  maxOutputTokens: number
  maxPromptBytes: number
  maxTargets: number
}

export interface ModelLabTarget {
  model: string
  providerDisplayName: string
  providerName: string
  providerType: ModelLabProviderType
  protocol: ModelLabProtocol
}

export interface ModelLabTicketRequest {
  targets: Array<Pick<ModelLabTarget, 'model' | 'providerName'>>
}

export interface ModelLabTicket {
  endpoint: string
  expiresAt?: string
  method: 'POST'
  model: string
  protocol: ModelLabProtocol
  providerName: string
  token: string
}

export interface ModelLabTicketResponse {
  comparisonId: string
  expiresAt: string
  tickets: ModelLabTicket[]
}

export type ModelLabLaneStatus =
  | 'idle'
  | 'authorizing'
  | 'streaming'
  | 'completed'
  | 'error'
  | 'stopped'

export interface ModelLabUsage {
  inputTokens: number | null
  outputTokens: number | null
}

export interface ModelLabLane {
  clientDurationMs: number | null
  clientTtftMs: number | null
  error: string | null
  id: string
  output: string
  status: ModelLabLaneStatus
  target: ModelLabTarget
  usage: ModelLabUsage
}

export type ModelLabConversationRole = 'assistant' | 'user'

export interface ModelLabConversationMessage {
  content: string
  role: ModelLabConversationRole
}

export interface ModelLabRunSettings {
  maxOutputTokens: number
  temperature: number
}

export interface ModelLabTurn {
  comparisons: ModelLabLane[]
  context: ModelLabConversationMessage[]
  id: string
  omittedTurns: number
  primary: ModelLabLane
  prompt: string
  settings: ModelLabRunSettings
}

export interface ModelLabContextWindow {
  byteLength: number
  fits: boolean
  messages: ModelLabConversationMessage[]
  omittedTurns: number
}

export interface ModelLabStreamUpdate {
  output?: string
  usage?: Partial<ModelLabUsage>
}
