import type { AdminUser } from '~/types/users'
import { downloadCsv, rowsToCsv } from './csv'

const USER_CSV_HEADERS = [
  'ID',
  'Name',
  'Username',
  'Email',
  'Role',
  'Access',
  'Platform activity (30d)',
  'Subscription plan',
  'Expires at',
  'Last login at',
  'Input tokens',
  'Output tokens',
  'Created at',
  'Updated at',
]

// usersToCsv serializes admin users into a spreadsheet-safe CSV document.
export function usersToCsv(users: AdminUser[]) {
  const rows = users.map((user) => [
    user.id,
    user.name,
    user.preferredUsername,
    user.email,
    user.role,
    user.isActive ? 'Enabled' : 'Disabled',
    user.activeInLast30Days ? 'Active' : 'Inactive',
    user.effectiveSubscriptionPlan?.name ?? user.subscriptionPlan?.name ?? '',
    user.expiresAt,
    user.lastLoginAt ?? 'Never',
    user.inputTokens,
    user.outputTokens,
    user.createdAt,
    user.updatedAt,
  ])

  return rowsToCsv([USER_CSV_HEADERS, ...rows])
}

// downloadUsersCsv triggers a UTF-8 CSV download compatible with spreadsheets.
export function downloadUsersCsv(users: AdminUser[], filename: string) {
  downloadCsv(usersToCsv(users), filename)
}
