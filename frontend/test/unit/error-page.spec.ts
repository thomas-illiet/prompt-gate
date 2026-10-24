import { describe, expect, it } from 'vitest'

import { getErrorPageContent } from '../../app/utils/error-page'

describe('getErrorPageContent', () => {
  it.each([
    [401, 'Session expired', 'forbidden'],
    [403, 'Access denied', 'forbidden'],
    [404, 'Page not found', 'not-found'],
    [408, 'The signal timed out', 'server'],
    [429, 'Traffic is taking a breather', 'rate-limit'],
    [503, 'Service temporarily offline', 'server'],
  ] as const)(
    'maps HTTP %i to a safe product state',
    (code, title, variant) => {
      const content = getErrorPageContent(code)

      expect(content).toMatchObject({ code, title, variant })
    },
  )

  it('uses a generic server state without leaking an error message', () => {
    expect(getErrorPageContent(500)).toMatchObject({
      code: 500,
      retry: true,
      title: 'Something went off course',
      variant: 'server',
    })
  })

  it('does not promise that retrying a timed-out mutation is safe', () => {
    const content = getErrorPageContent(408)

    expect(content.text).toContain('Check the current state')
    expect(content.text).not.toContain('safe to retry')
  })
})
