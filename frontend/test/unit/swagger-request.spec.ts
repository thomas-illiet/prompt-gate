import { describe, expect, it } from 'vitest'

import { rebaseSwaggerRequest } from '../../app/utils/swagger-request'

describe('rebaseSwaggerRequest', () => {
  it('preserves path, query, and request options on the configured API base', () => {
    const request = {
      credentials: 'include',
      headers: { Accept: 'application/json' },
      method: 'GET',
      url: '/api/v1/me/dashboard/tokens?window=30d&metric=tokens',
    }

    expect(
      rebaseSwaggerRequest(
        request,
        'https://api.example.test/gateway/',
        'https://frontend.example.test',
      ),
    ).toEqual({
      credentials: 'include',
      headers: { Accept: 'application/json' },
      method: 'GET',
      url: 'https://api.example.test/gateway/api/v1/me/dashboard/tokens?window=30d&metric=tokens',
    })
    expect(request.url).toBe(
      '/api/v1/me/dashboard/tokens?window=30d&metric=tokens',
    )
  })

  it('rebases absolute Swagger URLs without copying their origin', () => {
    expect(
      rebaseSwaggerRequest(
        {
          method: 'POST',
          url: 'https://frontend.example.test/api/v1/tokens?dryRun=true',
        },
        'https://api.example.test',
        'https://frontend.example.test',
      ).url,
    ).toBe('https://api.example.test/api/v1/tokens?dryRun=true')
  })
})
