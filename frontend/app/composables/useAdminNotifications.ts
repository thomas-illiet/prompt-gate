import type {
  NotificationAudiencePayload,
  NotificationAudienceResult,
  NotificationDeliveryStatus,
  NotificationKind,
  NotificationMessageDetail,
  NotificationMessageListResponse,
  NotificationMessageSummary,
  NotificationPreview,
  SendNotificationPayload,
} from '~/types/notifications'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_email_address: 'Enter one valid email address.',
  invalid_email_audience: 'Select at least one valid recipient.',
  invalid_email_body: 'Message content is required.',
  invalid_email_subject:
    'Subject is required and must not contain line breaks.',
  invalid_idempotency_key: 'The send request could not be identified safely.',
  notification_not_found: 'This notification no longer exists.',
  notifications_disabled: 'Email notifications are disabled in configuration.',
}

export function toAdminNotificationErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected email notification error.',
  )
}

export function useAdminNotifications() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()
  const loading = shallowRef(false)
  const saving = shallowRef(false)
  const error = shallowRef('')
  const items = shallowRef<NotificationMessageSummary[]>([])
  const selectedMessage = shallowRef<NotificationMessageDetail | null>(null)
  const page = shallowRef(1)
  const pageSize = shallowRef(20)
  const total = shallowRef(0)
  const search = shallowRef('')
  const kind = shallowRef<NotificationKind | ''>('')
  const status = shallowRef<NotificationDeliveryStatus | ''>('')
  let historyRequest = 0
  let historyController: AbortController | null = null

  async function loadHistory() {
    const request = ++historyRequest
    historyController?.abort()
    const controller = new AbortController()
    historyController = controller
    loading.value = true
    error.value = ''
    try {
      const params = new URLSearchParams({
        page: String(page.value),
        pageSize: String(pageSize.value),
        sortBy: 'createdAt',
        sortDir: 'desc',
      })
      if (search.value.trim()) params.set('search', search.value.trim())
      if (kind.value) params.set('kind', kind.value)
      if (status.value) params.set('status', status.value)
      const result = await apiFetch<NotificationMessageListResponse>(
        `/api/v1/admin/mail/messages?${params}`,
        { signal: controller.signal },
      )
      if (request !== historyRequest) return
      items.value = result.items
      total.value = result.total
    } catch (fetchError) {
      if (!controller.signal.aborted && request === historyRequest) {
        error.value = toAdminNotificationErrorMessage(fetchError)
      }
    } finally {
      if (request === historyRequest) {
        loading.value = false
        historyController = null
      }
    }
  }

  async function resolveAudience(audience: NotificationAudiencePayload) {
    return await apiJson<NotificationAudienceResult>(
      '/api/v1/admin/mail/audience',
      audience,
      { method: 'POST' },
    )
  }

  async function preview(subject: string, markdown: string) {
    return await apiJson<NotificationPreview>(
      '/api/v1/admin/mail/preview',
      { subject, markdown },
      { method: 'POST' },
    )
  }

  async function send(payload: SendNotificationPayload) {
    saving.value = true
    try {
      return await apiJson<{
        audience: NotificationAudienceResult
        message: NotificationMessageDetail
      }>('/api/v1/admin/mail/messages', payload, {
        method: 'POST',
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      })
    } finally {
      saving.value = false
    }
  }

  async function loadMessage(id: string) {
    selectedMessage.value = await apiFetch<NotificationMessageDetail>(
      `/api/v1/admin/mail/messages/${id}`,
    )
    return selectedMessage.value
  }

  async function retryFailed(id: string) {
    saving.value = true
    try {
      await apiFetch(`/api/v1/admin/mail/messages/${id}/retry-failed`, {
        method: 'POST',
      })
      await Promise.all([loadMessage(id), loadHistory()])
    } finally {
      saving.value = false
    }
  }

  function applyFilters() {
    page.value = 1
    void loadHistory()
  }

  return {
    applyFilters,
    error,
    items,
    kind,
    loadHistory,
    loadMessage,
    loading,
    page,
    pageSize,
    preview,
    resolveAudience,
    retryFailed,
    saving,
    search,
    selectedMessage,
    send,
    status,
    total,
  }
}
