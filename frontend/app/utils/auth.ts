import type { AppRole, AuthUser } from '~/types/auth'

export const DEFAULT_AUTH_REDIRECT_PATH = '/dashboard'
export const BLOCKED_ROUTE_PATH = '/access-denied'
export const APP_ROLES: AppRole[] = ['none', 'user', 'manager', 'admin']
export const FRONTEND_ORIGIN_QUERY_PARAM = 'frontend_origin'

// isAuthUser validates the minimum session shape before it reaches route guards.
export function isAuthUser(value: unknown): value is AuthUser {
  if (typeof value !== 'object' || value === null) {
    return false
  }

  const candidate = value as Partial<AuthUser>
  return (
    typeof candidate.id === 'string' &&
    typeof candidate.sub === 'string' &&
    typeof candidate.preferredUsername === 'string' &&
    typeof candidate.email === 'string' &&
    typeof candidate.name === 'string' &&
    typeof candidate.isActive === 'boolean' &&
    typeof candidate.lastLoginAt === 'string' &&
    typeof candidate.role === 'string' &&
    APP_ROLES.includes(candidate.role as AppRole)
  )
}

const OIDC_CALLBACK_PARAMS = new Set([
  'code',
  'error',
  'error_description',
  'error_uri',
  'iss',
  'kc_action',
  'kc_action_status',
  'session_state',
  'state',
])

// sanitizeOidcSearchParams removes OIDC callback parameters from a URL query.
function sanitizeOidcSearchParams(params: URLSearchParams) {
  for (const param of OIDC_CALLBACK_PARAMS) {
    params.delete(param)
  }
}

// sanitizeOidcHash removes OIDC callback parameters from hash query fragments.
function sanitizeOidcHash(hash: string) {
  if (!hash.startsWith('#')) {
    return ''
  }

  const rawHash = hash.slice(1)
  if (!rawHash.includes('=')) {
    return hash
  }

  const params = new URLSearchParams(rawHash)
  sanitizeOidcSearchParams(params)

  const nextHash = params.toString()
  return nextHash ? `#${nextHash}` : ''
}

// currentFrontendOrigin returns the browser origin when running on the client.
export function currentFrontendOrigin() {
  if (!import.meta.client) {
    return ''
  }

  return window.location.origin
}

// resolveRuntimeApiBaseUrl normalizes the configured backend API base URL.
export function resolveRuntimeApiBaseUrl(
  value: unknown,
  currentOrigin: string | null = currentFrontendOrigin() || null,
) {
  if (typeof value === 'string' && value.trim().length > 0) {
    return value.trim().replace(/\/$/, '')
  }

  if (typeof currentOrigin !== 'string' || currentOrigin.trim().length === 0) {
    return ''
  }

  return currentOrigin.trim().replace(/\/$/, '')
}

// sanitizeRedirectPath keeps redirects local and removes OIDC callback noise.
export function sanitizeRedirectPath(
  value: unknown,
  fallback = DEFAULT_AUTH_REDIRECT_PATH,
) {
  if (typeof value !== 'string' || value.length === 0) {
    return fallback
  }

  if (!value.startsWith('/') || value.startsWith('//')) {
    return fallback
  }

  const url = new URL(value, 'http://localhost')
  sanitizeOidcSearchParams(url.searchParams)
  url.hash = sanitizeOidcHash(url.hash)

  const sanitizedPath = `${url.pathname}${url.search}${url.hash}`
  return sanitizedPath.startsWith('/') ? sanitizedPath : fallback
}

// normalizeRedirectPath normalizes a local redirect path with a fallback.
export function normalizeRedirectPath(
  value: unknown,
  fallback = DEFAULT_AUTH_REDIRECT_PATH,
) {
  return sanitizeRedirectPath(value, fallback)
}

// isBlockedUser reports whether a user should be blocked from protected areas.
export function isBlockedUser(user: AuthUser | null | undefined) {
  if (!user) {
    return false
  }

  return !user.isActive || user.role === 'none'
}

// hasRequiredRole checks whether a user satisfies route role requirements.
export function hasRequiredRole(
  user: AuthUser | null | undefined,
  requiredRoles?: AppRole[],
) {
  if (!requiredRoles || requiredRoles.length === 0) {
    return true
  }

  if (!user) {
    return false
  }

  return requiredRoles.includes(user.role)
}

// appRoleLabel returns the display label for an app role.
export function appRoleLabel(role: AppRole) {
  switch (role) {
    case 'admin':
      return 'Admin'
    case 'manager':
      return 'Manager'
    case 'user':
      return 'User'
    default:
      return 'None'
  }
}

// appRoleColor returns the Vuetify color used for an app role.
export function appRoleColor(role: AppRole) {
  switch (role) {
    case 'admin':
      return 'error'
    case 'manager':
      return 'warning'
    case 'user':
      return 'primary'
    default:
      return 'grey'
  }
}
