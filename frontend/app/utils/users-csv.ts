import type { AdminUser } from '~/types/users'

const USER_CSV_HEADERS = [
  'ID',
  'Name',
  'Username',
  'Email',
  'Role',
  'Status',
  'Subscription plan',
  'Expires at',
  'Last login at',
  'Input tokens',
  'Output tokens',
  'Created at',
  'Updated at',
]

// csvCell escapes spreadsheet control characters, quotes, and line breaks.
function csvCell(value: string | number | null | undefined) {
  let text = value == null ? '' : String(value)
  if (/^[=+\-@]/.test(text)) text = `'${text}`
  return `"${text.replaceAll('"', '""')}"`
}

// usersToCsv serializes admin users into a spreadsheet-safe CSV document.
export function usersToCsv(users: AdminUser[]) {
  const rows = users.map((user) => [
    user.id,
    user.name,
    user.preferredUsername,
    user.email,
    user.role,
    user.isActive ? 'Active' : 'Inactive',
    user.effectiveSubscriptionPlan?.name ?? user.subscriptionPlan?.name ?? '',
    user.expiresAt,
    user.lastLoginAt,
    user.inputTokens,
    user.outputTokens,
    user.createdAt,
    user.updatedAt,
  ])

  return [USER_CSV_HEADERS, ...rows]
    .map((row) => row.map(csvCell).join(','))
    .join('\r\n')
}

// downloadUsersCsv triggers a UTF-8 CSV download compatible with spreadsheets.
export function downloadUsersCsv(users: AdminUser[], filename: string) {
  const blob = new Blob([`\uFEFF${usersToCsv(users)}`], {
    type: 'text/csv;charset=utf-8',
  })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}
