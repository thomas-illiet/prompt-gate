export type NotificationKind =
  | 'welcome'
  | 'quota_exhausted'
  | 'quota_reset_granted'
  | 'group_changed'
  | 'token_created'
  | 'token_expired'
  | 'token_revoked'
  | 'monthly_usage_report'
  | 'manual'

export type NotificationDeliveryStatus =
  'pending' | 'processing' | 'sent' | 'failed'

export type NotificationAudienceMode = 'external' | 'users' | 'groups' | 'all'

export interface NotificationRecipient {
  userId: string | null
  email: string
  name: string
}

export interface NotificationAudiencePayload {
  mode: NotificationAudienceMode
  externalAddress: string
  userIds: string[]
  groupIds: string[]
}

export interface NotificationAudienceResult {
  recipients: NotificationRecipient[]
  valid: number
  ignored: number
}

export interface NotificationMessageSummary {
  id: string
  kind: NotificationKind
  subject: string
  audience: string
  recipients: number
  pending: number
  processing: number
  sent: number
  failed: number
  createdAt: string
}

export interface NotificationDelivery {
  id: string
  userId: string | null
  recipientEmail: string
  recipientName: string
  status: NotificationDeliveryStatus
  attempts: number
  nextAttemptAt: string
  sentAt: string | null
  lastAttemptAt: string | null
  lastError: string
}

export interface NotificationMessageDetail extends NotificationMessageSummary {
  markdownBody: string
  textBody: string
  htmlBody: string
  deliveries: NotificationDelivery[]
}

export interface NotificationMessageListResponse {
  items: NotificationMessageSummary[]
  page: number
  pageSize: number
  total: number
}

export interface NotificationPreview {
  textBody: string
  htmlBody: string
}

export interface SendNotificationPayload {
  subject: string
  markdown: string
  audience: NotificationAudiencePayload
}
