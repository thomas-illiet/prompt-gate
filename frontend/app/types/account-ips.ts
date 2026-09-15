export interface AccountIPAddress {
  ip: string
  lastSeen: string
}

export interface AccountIPAddressListResponse {
  items: AccountIPAddress[]
  page: number
  pageSize: number
  total: number
}
