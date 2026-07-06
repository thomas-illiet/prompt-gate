export type FirewallAction = 'allow' | 'deny'
export type FirewallMoveDirection = 'increase' | 'decrease'

export interface FirewallRule {
  id: string
  serviceAccountId?: string
  userId?: string
  address: string
  description: string
  priority: number
  action: FirewallAction
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface FirewallRuleListResponse {
  items: FirewallRule[]
  page: number
  pageSize: number
  total: number
}

export interface FirewallRulePayload {
  address: string
  description: string
  priority: number
  action: FirewallAction
  enabled: boolean
}

export interface FirewallSimulationResponse {
  allowed: boolean
  matchedRule: FirewallRule | null
}
