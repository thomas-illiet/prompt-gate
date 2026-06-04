import type { AppRole } from '~/types/auth'
import type {
  AccountQuotaState,
  AccountSubscriptionPlan,
} from '~/types/subscriptions'

export interface ServiceAccount {
  id: string
  identifier: string
  name: string
  role: AppRole
  subscriptionPlanId?: string | null
  subscriptionPlan?: AccountSubscriptionPlan
  effectiveSubscriptionPlan?: AccountSubscriptionPlan
  quotaState?: AccountQuotaState
  note: string
  isActive: boolean
  firewallOverrideEnabled: boolean
  inputTokens: number
  outputTokens: number
  createdAt: string
  updatedAt: string
}

export interface ServiceAccountListResponse {
  items: ServiceAccount[]
  page: number
  pageSize: number
  total: number
}

export interface ServiceAccountPayload {
  identifier: string
  name: string
  isActive: boolean
  firewallOverrideEnabled?: boolean
}

export interface ServiceAccountFormPayload extends ServiceAccountPayload {
  subscriptionPlanId: string | null
}

export interface TokenResponse {
  id: string
  userId: string
  name: string
  description: string
  expiresAt: string
  createdAt: string
  revokedAt?: string
  expiredAt?: string
}

export interface TokenListResponse {
  items: TokenResponse[]
  page: number
  pageSize: number
  total: number
}

export interface TokenPayload {
  name: string
  description: string
  expiresInDays?: number
}

export interface CreatedTokenResponse {
  token: string
  tokenInfo: TokenResponse
}
