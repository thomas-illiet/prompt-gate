import {
  BLOCKED_ROUTE_PATH,
  DEFAULT_AUTH_REDIRECT_PATH,
  hasRequiredRole,
  isBlockedUser,
  normalizeRedirectPath,
  sanitizeRedirectPath,
} from '~/utils/auth'

export default defineNuxtRouteMiddleware(async (to) => {
  const authStore = useAuthStore()
  await authStore.waitUntilReady()

  const isPublicRoute = to.meta.auth === false || to.path === '/login'
  const allowBlocked =
    to.meta.allowBlocked === true || to.path === BLOCKED_ROUTE_PATH
  const requiredRoles = Array.isArray(to.meta.requiredRoles)
    ? to.meta.requiredRoles
    : []
  const redirectTarget = sanitizeRedirectPath(
    to.fullPath,
    DEFAULT_AUTH_REDIRECT_PATH,
  )

  if (isPublicRoute) {
    if (to.path === '/login' && authStore.isAuthenticated) {
      if (isBlockedUser(authStore.user)) {
        return navigateTo(BLOCKED_ROUTE_PATH)
      }

      return navigateTo(
        normalizeRedirectPath(to.query.redirect, DEFAULT_AUTH_REDIRECT_PATH),
      )
    }

    return
  }

  if (!authStore.isAuthenticated) {
    return navigateTo({
      path: '/login',
      query: { redirect: redirectTarget },
    })
  }

  if (isBlockedUser(authStore.user) && !allowBlocked) {
    return navigateTo(BLOCKED_ROUTE_PATH)
  }

  if (
    requiredRoles.length > 0 &&
    !hasRequiredRole(authStore.user, requiredRoles)
  ) {
    return navigateTo(BLOCKED_ROUTE_PATH)
  }
})
