export interface PriceRates {
  input: number
  output: number
}

export interface ModelPriceRecord extends PriceRates {
  id?: string
  providerName: string
  model: string
  createdAt?: string
  updatedAt?: string
}

export interface PricingConfigResponse {
  fallback: PriceRates
  models: ModelPriceRecord[]
}

export interface MissingModelPrice {
  providerName: string
  model: string
}

export interface ProviderModelError {
  providerName: string
  message: string
}

export interface PricingCheckResponse {
  configured: boolean
  missingPrices: MissingModelPrice[]
  providerErrors: ProviderModelError[]
  checkedAt: string
}

export type ModelPricePayload = Omit<
  ModelPriceRecord,
  'createdAt' | 'id' | 'updatedAt'
>
