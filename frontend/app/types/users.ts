import type { AppRole, AuthUser } from '~/types/auth'

export interface AdminUser extends AuthUser {
  inputTokens: number
  outputTokens: number
  expiresAt: string | null
  createdAt: string
  updatedAt: string
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

export type UserRoleFilter = 'all' | AppRole
export type UserStatusFilter = 'all' | 'active' | 'inactive'
