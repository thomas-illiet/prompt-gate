export interface AccountIPAddress {
  ip: string
  lastSeen: string
  lastDeniedReason: string | null
  lastDeniedAt: string | null
}

export interface AccountIPAddressListResponse {
  items: AccountIPAddress[]
  page: number
  pageSize: number
  total: number
}
