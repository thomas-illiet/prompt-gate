function encodePathSegment(segment: string) {
  return encodeURIComponent(segment)
}

function joinApiPath(basePath: string, ...segments: string[]) {
  const normalizedBase = basePath.replace(/\/+$/, '')
  if (segments.length === 0) {
    return normalizedBase
  }

  return [
    normalizedBase,
    ...segments.map((segment) => encodePathSegment(segment)),
  ].join('/')
}

export function withApiQuery(path: string, params: URLSearchParams) {
  const queryString = params.toString()
  return queryString ? `${path}?${queryString}` : path
}

export const adminGroupsPath = '/api/v1/admin/groups'
export const adminServiceAccountsPath = '/api/v1/admin/service-accounts'
export const adminUsersPath = '/api/v1/admin/users'

export function adminGroupPath(groupId: string, ...segments: string[]) {
  return joinApiPath(adminGroupsPath, groupId, ...segments)
}

export function adminServiceAccountPath(
  accountId: string,
  ...segments: string[]
) {
  return joinApiPath(adminServiceAccountsPath, accountId, ...segments)
}

export function adminUserPath(userId: string, ...segments: string[]) {
  return joinApiPath(adminUsersPath, userId, ...segments)
}
