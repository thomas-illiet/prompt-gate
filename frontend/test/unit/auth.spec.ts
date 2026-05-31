import { describe, expect, it } from 'vitest'

import { resolveRuntimeApiBaseUrl } from '../../app/utils/auth'

describe('resolveRuntimeApiBaseUrl', () => {
  it('uses the configured API base URL when provided', () => {
    expect(
      resolveRuntimeApiBaseUrl(
        ' https://api.example.com/ ',
        'http://localhost:8080',
      ),
    ).toBe('https://api.example.com')
  })

  it('falls back to the current frontend origin for same-origin deployments', () => {
    expect(resolveRuntimeApiBaseUrl('', 'http://localhost:8080/')).toBe(
      'http://localhost:8080',
    )
  })

  it('returns an empty URL when neither source is available', () => {
    expect(resolveRuntimeApiBaseUrl('', null)).toBe('')
  })
})
