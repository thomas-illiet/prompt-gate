import type { AppRole, AuthUser } from '~/types/auth'
import type {
  AccountQuotaState,
  AccountSubscriptionPlan,
} from '~/types/subscriptions'

export interface AdminUser extends Omit<AuthUser, 'lastLoginAt' | 'sub'> {
  type: 'user'
  sub: string | null
  lastLoginAt: string | null
  activeInLast30Days: boolean
  quotaResetCredits: number
  subscriptionPlanId: string | null
  subscriptionPlan?: AccountSubscriptionPlan
  effectiveSubscriptionPlan?: AccountSubscriptionPlan
  quotaState?: AccountQuotaState
  note: string
  inputTokens: number
  outputTokens: number
  expiresAt: string | null
  createdAt: string
  updatedAt: string
}

export interface CreateUserPayload {
  preferredUsername: string
}

export interface UserListResponse {
  items: AdminUser[]
  page: number
  pageSize: number
  total: number
}

export interface UpdateUserPayload {
  role: AppRole
  isActive: boolean
  expiresAt: string | null
}

export interface UpdateUserAccessPayload extends UpdateUserPayload {
  subscriptionPlanId: string | null
}

export type UserRoleFilter = 'all' | AppRole
export type UserStatusFilter = 'all' | 'active' | 'inactive'
