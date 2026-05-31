import { FetchError } from 'ofetch'
import { describe, expect, it } from 'vitest'

import {
  apiErrorCode,
  isApiError,
  toApiErrorMessage,
} from '../../app/utils/api-error'

function apiError(code: string) {
  return Object.assign(Object.create(FetchError.prototype), {
    response: {
      _data: { error: code },
    },
  }) as FetchError
}

describe('api error helpers', () => {
  it('detects and extracts backend error codes', () => {
    expect(isApiError({ error: 'invalid_name' })).toBe(true)
    expect(isApiError({ error: 42 })).toBe(false)
    expect(apiErrorCode(apiError('invalid_name'))).toBe('invalid_name')
  })

  it('maps known API codes and returns unknown codes verbatim', () => {
    expect(
      toApiErrorMessage(
        apiError('invalid_name'),
        { invalid_name: 'Name is invalid.' },
        'Fallback error.',
      ),
    ).toBe('Name is invalid.')

    expect(toApiErrorMessage(apiError('new_code'), {}, 'Fallback error.')).toBe(
      'new_code',
    )
  })

  it('handles Error, string, and unknown values', () => {
    expect(toApiErrorMessage(new Error('Boom'), {}, 'Fallback error.')).toBe(
      'Boom',
    )
    expect(toApiErrorMessage('Plain error', {}, 'Fallback error.')).toBe(
      'Plain error',
    )
    expect(toApiErrorMessage({ nope: true }, {}, 'Fallback error.')).toBe(
      'Fallback error.',
    )
  })
})
