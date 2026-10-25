import type { CurrentQuotaStatus } from '~/types/subscriptions'

export type QuotaResetHistoryType =
  'monthly_grant' | 'admin_grant' | 'quota_reset_used'

export type AdminQuotaAccountType = 'user' | 'service'
export type AdminQuotaAccountTypeFilter = 'all' | AdminQuotaAccountType
export type AdminQuotaStateStatus =
  'unassigned' | 'unlimited' | 'healthy' | 'warning' | 'critical'
export type AdminQuotaStatesView = 'at-risk' | 'all'
export type AdminQuotaWindow = '5h' | '7d'
export type AdminQuotaStatesSortBy =
  'saturation' | 'displayName' | 'resetAt' | 'syncedAt'

export interface AdminQuotaWindowState {
  window: AdminQuotaWindow
  usedTokens: number
  quotaTokens: number | null
  remainingTokens: number | null
  percent: number | null
  resetAt: string | null
}

export interface AdminQuotaStateItem {
  accountId: string
  accountType: AdminQuotaAccountType
  displayName: string
  planName: string | null
  hasSubscription: boolean
  status: AdminQuotaStateStatus
  maximumPercent: number | null
  mostSaturatedWindow: AdminQuotaWindow | null
  nextResetAt: string | null
  windows: AdminQuotaWindowState[]
  syncedAt: string | null
}

export interface AdminQuotaStatesSummary {
  projectedAccounts: number
  atRiskAccounts: number
  exhaustedAccounts: number
}

export interface AdminQuotaStatesResponse {
  generatedAt: string
  projectionSyncedAt: string | null
  summary: AdminQuotaStatesSummary
  items: AdminQuotaStateItem[]
  page: number
  pageSize: number
  total: number
}

export type AdminQuotaResetEventWindow = '7d' | '30d' | 'all'
export type AdminQuotaResetEventStatus = 'pending' | 'applied'
export type AdminQuotaResetEventTypeFilter = 'all' | QuotaResetHistoryType
export type AdminQuotaResetEventStatusFilter =
  'all' | AdminQuotaResetEventStatus
export type AdminQuotaResetEventsSortBy =
  'createdAt' | 'appliedAt' | 'user' | 'type' | 'status'
export type AdminQuotaResetActorType =
  'system' | 'user' | 'admin_session' | 'admin_api_key'

export interface AdminQuotaResetActor {
  type: AdminQuotaResetActorType
  userId: string | null
  displayName: string | null
}

export interface AdminQuotaResetEventItem {
  id: string
  target: {
    accountId: string
    accountType: AdminQuotaAccountType
    displayName: string
  }
  type: QuotaResetHistoryType
  status: AdminQuotaResetEventStatus
  delta: number
  balanceAfter: number
  actor: AdminQuotaResetActor
  grantMonth: string | null
  resetCutoffAt: string | null
  reconcileAttempts: number
  lastReconcileAttemptAt: string | null
  appliedAt: string | null
  createdAt: string
}

export interface AdminQuotaResetEventsSummary {
  monthlyGrants: number
  adminGrants: number
  appliedResetUses: number
  pendingResetUsesTotal: number
}

export interface AdminQuotaResetEventsResponse {
  window: AdminQuotaResetEventWindow
  startsAt: string | null
  endsAt: string
  summary: AdminQuotaResetEventsSummary
  items: AdminQuotaResetEventItem[]
  page: number
  pageSize: number
  total: number
}

export interface QuotaResetSummary {
  available: number
  maximum: number
  hasResettableQuota: boolean
  pendingReset: boolean
}

export interface QuotaResetHistoryItem {
  id: string
  type: QuotaResetHistoryType
  delta: number
  balanceAfter: number
  createdAt: string
}

export interface QuotaResetHistoryResponse {
  items: QuotaResetHistoryItem[]
  page: number
  pageSize: number
  total: number
}

export type UseQuotaResetResponse =
  | {
      status: 'applied'
      summary: QuotaResetSummary
      quota: CurrentQuotaStatus
    }
  | {
      status: 'pending'
      summary: QuotaResetSummary
      quota?: never
    }

export const QUOTA_RESET_MAXIMUM = 2
