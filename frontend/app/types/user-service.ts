export type UsageDays = 7 | 30
export type UsageWindow = '7d' | '30d' | 'all'
export type DashboardScope = 'self' | 'global'

export interface UserToken {
  id: string
  userId: string
  name: string
  description: string
  expiresAt: string
  createdAt: string
  revokedAt?: string
  expiredAt?: string
}

export interface UserTokenPayload {
  name: string
  description: string
  expiresInDays?: number
}

export interface CreatedUserToken {
  token: string
  tokenInfo: UserToken
}

export interface UserTokenListResponse {
  items: UserToken[]
  page: number
  pageSize: number
  total: number
}

export type UserTokenStatus = 'active' | 'expired' | 'revoked'
export type UserTokenStatusFilter = 'all' | UserTokenStatus

export interface UserTokenStats {
  all: number
  active: number
  expired: number
  revoked: number
}

export interface CostRates {
  inputUsdPer1MTokens: number
  outputUsdPer1MTokens: number
  embeddingUsdPer1MTokens: number
}

export interface EstimatedCost {
  inputUsd: number
  outputUsd: number
  embeddingUsd: number
  totalUsd: number
  rates: CostRates
}

export interface UsageTotals {
  requests: number
  prompts: number
  toolCalls: number
  inputTokens: number
  outputTokens: number
  cacheReadInputTokens: number
  cacheWriteInputTokens: number
  completionInputTokens: number
  completionOutputTokens: number
  completionTokens: number
  embeddingTokens: number
  totalTokens: number
  estimatedCost?: EstimatedCost
}

export interface UsageWindowMeta {
  window: UsageWindow
  startsAt: string
  endsAt: string
}

export interface DailyUsage {
  date: string
  requests: number
  prompts: number
  inputTokens: number
  outputTokens: number
  completionInputTokens: number
  completionOutputTokens: number
  completionTokens: number
  embeddingTokens: number
  totalTokens: number
  estimatedCost?: EstimatedCost
}

export interface UsageBreakdown {
  name: string
  requests: number
  totalTokens: number
  estimatedCost?: EstimatedCost
}

export interface PromptHistoryItem {
  id: string
  interceptionId: string
  providerResponseId: string
  provider: string
  providerType: string
  model: string
  prompt: string
  inputTokens: number
  outputTokens: number
  totalTokens: number
  durationMs: number | null
  createdAt: string
}

export interface AdminPromptHistoryItem extends PromptHistoryItem {
  userId: string
  userName: string
  userEmail: string
  userPreferredUsername: string
}

export interface PromptHistoryResponse {
  items: PromptHistoryItem[]
  page: number
  pageSize: number
  total: number
}

export interface AdminPromptHistoryResponse {
  items: AdminPromptHistoryItem[]
  page: number
  pageSize: number
  total: number
}

export interface UserUsageSummary {
  days: UsageDays
  startsAt: string
  endsAt: string
  totals: UsageTotals
  daily: DailyUsage[]
  topModels: UsageBreakdown[]
  topProviders: UsageBreakdown[]
  recentPrompts: PromptHistoryItem[]
}

export interface DashboardTokensResponse extends UsageWindowMeta {
  inputTokens: number
  outputTokens: number
  cacheReadInputTokens: number
  cacheWriteInputTokens: number
  completionInputTokens: number
  completionOutputTokens: number
  completionTokens: number
  embeddingTokens: number
  totalTokens: number
  estimatedCost?: EstimatedCost
}

export interface DashboardMessagesResponse extends UsageWindowMeta {
  messages: number
}

export interface DashboardDurationResponse extends UsageWindowMeta {
  totalDurationMs: number
}

export interface DashboardActivityResponse extends UsageWindowMeta {
  daily: DailyUsage[]
}

export interface DashboardBreakdownResponse extends UsageWindowMeta {
  items: UsageBreakdown[]
}

export interface DashboardAdoptionResponse extends UsageWindowMeta {
  activeUsers: number
  activeServiceAccounts: number
  activeVirtualKeys: number
}

export type HelpProviderType = 'openai' | 'anthropic' | 'ollama'

export interface HelpSetupProvider {
  name: string
  displayName: string
  type: HelpProviderType
  routePrefix: string
  openaiBaseUrl?: string
  anthropicBaseUrl?: string
  models: string[]
  modelsError?: string
}

export interface HelpSetupResponse {
  proxyBaseUrl: string
  providers: HelpSetupProvider[]
}
