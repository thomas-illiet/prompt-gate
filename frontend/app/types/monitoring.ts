export type MonitoringStatus = 'ok' | 'degraded'

export interface MonitoringService {
  id: string
  name: string
  displayName: string
  url: string
  expectedStatusCode: number
  intervalSeconds: number
  enabled: boolean
  status: MonitoringStatus
  lastCheckedAt: string | null
  lastStatusCode: number | null
  lastError: string
  lastLatencyMs: number
  consecutiveFailures: number
  createdAt: string
  updatedAt: string
}

export interface MonitoringServiceListResponse {
  items: MonitoringService[]
  page: number
  pageSize: number
  total: number
}

export interface MonitoringServicePayload {
  name: string
  displayName: string
  url: string
  expectedStatusCode: number
  intervalSeconds: number
  enabled: boolean
}

export interface MonitoringStatusService {
  id: string
  name: string
  displayName: string
  status: MonitoringStatus
  lastCheckedAt: string | null
  lastStatusCode: number | null
  lastError: string
  lastLatencyMs: number
}

export interface MonitoringStatusResponse {
  status: MonitoringStatus
  services: MonitoringStatusService[]
}
