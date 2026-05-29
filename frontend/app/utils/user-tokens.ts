import type { UserToken, UserTokenStatus } from '~/types/user-service'

// userTokenStatus derives the current status for a user token.
export function userTokenStatus(
  token: UserToken,
  now: Date = new Date(),
): UserTokenStatus {
  if (token.revokedAt) {
    return 'revoked'
  }

  if (token.expiredAt || new Date(token.expiresAt).getTime() <= now.getTime()) {
    return 'expired'
  }

  return 'active'
}

// userTokenStatusLabel returns the display label for a token status.
export function userTokenStatusLabel(status: UserTokenStatus) {
  switch (status) {
    case 'active':
      return 'Active'
    case 'expired':
      return 'Expired'
    case 'revoked':
      return 'Revoked'
  }
}

// userTokenStatusColor returns the Vuetify color for a token status.
export function userTokenStatusColor(status: UserTokenStatus) {
  switch (status) {
    case 'active':
      return 'success'
    case 'expired':
      return 'warning'
    case 'revoked':
      return 'grey'
  }
}

// canRevokeUserToken reports whether the token can still be revoked.
export function canRevokeUserToken(token: UserToken, now: Date = new Date()) {
  return userTokenStatus(token, now) === 'active'
}
