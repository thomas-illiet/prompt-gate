export interface SubscriptionPlan {
  id: string
  name: string
  description: string
  quota5hTokens: number | null
  quota7dTokens: number | null
  isDefault: boolean
  assignedUsersCount: number
  assignedServiceAccountsCount: number
  assignedDirectAccountsCount: number
  assignedIndirectAccountsCount: number
  assignedAccountsCount: number
  createdAt: string
  updatedAt: string
}

export interface SubscriptionPlanListResponse {
  items: SubscriptionPlan[]
  page: number
  pageSize: number
  total: number
}

export interface SubscriptionPlanPayload {
  name: string
  description: string
  quota5hTokens: number | null
  quota7dTokens: number | null
  isDefault: boolean
}

export interface AssignSubscriptionPlanPayload {
  planId: string | null
}

export interface AccountSubscriptionPlan {
  id: string
  name: string
  description: string
  quota5hTokens: number | null
  quota7dTokens: number | null
  isDefault: boolean
}

export interface AccountQuotaState {
  hasSubscription: boolean
  planId: string | null
  planName: string
  used5hTokens: number
  quota5hTokens: number | null
  reset5hAt: string | null
  used7dTokens: number
  quota7dTokens: number | null
  reset7dAt: string | null
  syncedAt: string | null
}

export interface CurrentQuotaStatus {
  hasSubscription: boolean
  plan?: SubscriptionPlan
  used5hTokens: number
  quota5hTokens: number | null
  remaining5hTokens: number | null
  reset5hAt: string | null
  used7dTokens: number
  quota7dTokens: number | null
  remaining7dTokens: number | null
  reset7dAt: string | null
  syncedAt?: string | null
}
