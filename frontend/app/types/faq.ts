export interface FAQEntry {
  id: string
  question: string
  answer: string
  renderedHtml: string
  position: number
  published: boolean
  createdAt: string
  updatedAt: string
}

export interface PublicFAQEntry {
  id: string
  question: string
  renderedHtml: string
  position: number
}

export interface FAQListResponse {
  items: FAQEntry[]
  page: number
  pageSize: number
  total: number
}

export interface FAQPayload {
  question: string
  answer: string
  published: boolean
}
