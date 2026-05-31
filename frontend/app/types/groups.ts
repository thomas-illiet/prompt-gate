import type { AppRole } from '~/types/auth'
import type { ProviderType } from '~/types/providers'

export type GroupMemberType = 'user' | 'service'

export interface GroupProviderSummary {
  id: string
  name: string
  displayName: string
  type: ProviderType
  enabled: boolean
}

export interface GroupMemberSummary {
  id: string
  preferredUsername: string
  email: string
  name: string
  type: GroupMemberType
  role: AppRole
  isActive: boolean
}

export interface AccessGroup {
  id: string
  name: string
  displayName: string
  description: string
  providers: GroupProviderSummary[]
  modelPatterns: string[]
  members: GroupMemberSummary[]
  providerCount: number
  modelPatternCount: number
  memberCount: number
  createdAt: string
  updatedAt: string
}

export interface ProfileGroupSummary {
  id: string
  name: string
  displayName: string
  description: string
}

export interface GroupListResponse {
  items: AccessGroup[]
  page: number
  pageSize: number
  total: number
}

export interface GroupPayload {
  name: string
  displayName: string
  description: string
  providerIds: string[]
  modelPatterns: string[]
}

export interface ReplaceUserGroupsPayload {
  groupIds: string[]
}

export interface GroupModelPatternValidationPayload {
  providerIds: string[]
  modelPatterns: string[]
}

export interface GroupModelPatternProviderValidation {
  id: string
  name: string
  displayName: string
  availableModelCount: number
  matchedModelCount: number
  matchedModels: string[]
  modelsError?: string
}

export interface GroupModelPatternValidationResponse {
  matchedModelCount: number
  matchedModels: string[]
  providerResults: GroupModelPatternProviderValidation[]
  unavailableProviderCount: number
}
