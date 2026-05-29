import { FetchError } from 'ofetch'

export type ApiErrorMessages = Readonly<Record<string, string>>

// isApiError checks whether an unknown payload contains an API error code.
export function isApiError(value: unknown): value is { error: string } {
  return (
    typeof value === 'object' &&
    value !== null &&
    'error' in value &&
    typeof value.error === 'string'
  )
}

// apiErrorCode extracts the backend error code from an ofetch error.
export function apiErrorCode(error: unknown) {
  if (!(error instanceof FetchError)) {
    return ''
  }

  const data = error.response?._data
  return isApiError(data) ? data.error : ''
}

// toApiErrorMessage resolves an API error into a display message.
export function toApiErrorMessage(
  error: unknown,
  messages: ApiErrorMessages,
  fallback: string,
) {
  const code = apiErrorCode(error)
  if (code) {
    return messages[code] ?? code
  }

  if (error instanceof Error) {
    return error.message
  }

  if (typeof error === 'string') {
    return error
  }

  return fallback
}
