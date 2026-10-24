export type AppErrorVariant =
  'forbidden' | 'generic' | 'not-found' | 'rate-limit' | 'server'

export interface ErrorPageContent {
  code: number
  eyebrow: string
  primaryLabel: string
  primaryRedirect: string
  retry: boolean
  text: string
  title: string
  variant: AppErrorVariant
}

const errorPages: Record<number, ErrorPageContent> = {
  401: {
    code: 401,
    eyebrow: 'Session checkpoint',
    primaryLabel: 'Sign in again',
    primaryRedirect: '/login',
    retry: false,
    text: 'Your session is no longer active. Sign in again to reopen the control room securely.',
    title: 'Session expired',
    variant: 'forbidden',
  },
  403: {
    code: 403,
    eyebrow: 'Access checkpoint',
    primaryLabel: 'Back to dashboard',
    primaryRedirect: '/dashboard',
    retry: false,
    text: 'You are signed in, but this area is outside your current permissions.',
    title: 'Access denied',
    variant: 'forbidden',
  },
  404: {
    code: 404,
    eyebrow: 'Route signal',
    primaryLabel: 'Back to dashboard',
    primaryRedirect: '/dashboard',
    retry: false,
    text: 'This route is not connected to PromptGate. Check the address or return to a known workspace view.',
    title: 'Page not found',
    variant: 'not-found',
  },
  408: {
    code: 408,
    eyebrow: 'Connection timeout',
    primaryLabel: 'Try again',
    primaryRedirect: '/',
    retry: true,
    text: 'The request timed out before PromptGate could confirm the outcome. Check the current state before trying again.',
    title: 'The signal timed out',
    variant: 'server',
  },
  429: {
    code: 429,
    eyebrow: 'Traffic checkpoint',
    primaryLabel: 'Try again',
    primaryRedirect: '/',
    retry: true,
    text: 'PromptGate is receiving too many requests right now. Give the queue a moment, then try again.',
    title: 'Traffic is taking a breather',
    variant: 'rate-limit',
  },
  503: {
    code: 503,
    eyebrow: 'Service signal',
    primaryLabel: 'Try again',
    primaryRedirect: '/',
    retry: true,
    text: 'The service is temporarily unavailable. The control room is intact; reconnect in a moment.',
    title: 'Service temporarily offline',
    variant: 'server',
  },
}

export function getErrorPageContent(statusCode: number): ErrorPageContent {
  const knownPage = errorPages[statusCode]
  if (knownPage) return knownPage

  if (statusCode >= 500) {
    return {
      code: statusCode,
      eyebrow: 'Control room signal',
      primaryLabel: 'Try again',
      primaryRedirect: '/',
      retry: true,
      text: 'PromptGate hit an unexpected problem. No technical details or request data are exposed here.',
      title: 'Something went off course',
      variant: 'server',
    }
  }

  return {
    code: statusCode,
    eyebrow: 'Request checkpoint',
    primaryLabel: 'Back to dashboard',
    primaryRedirect: '/dashboard',
    retry: false,
    text: 'PromptGate could not complete this request. Return to the dashboard and choose another route.',
    title: 'Request could not be completed',
    variant: 'generic',
  }
}
