import type { CurrentQuotaStatus } from '~/types/subscriptions'

export type QuotaResetHistoryType =
  'monthly_grant' | 'admin_grant' | 'quota_reset_used'

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

export interface UseQuotaResetResponse {
  status: 'applied' | 'pending'
  summary: QuotaResetSummary
  quota: CurrentQuotaStatus
}

export const QUOTA_RESET_MAXIMUM = 2
