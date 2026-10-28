<script setup lang="ts">
import AdminEmailComposer from '~/components/AdminNotifications/AdminEmailComposer.vue'
import type { AccessGroup } from '~/types/groups'
import type {
  NotificationAudiencePayload,
  NotificationAudienceResult,
  NotificationPreview,
  SendNotificationPayload,
} from '~/types/notifications'
import type { AdminUser } from '~/types/users'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Send email',
  icon: 'mdi-email-edit-outline',
  drawerIndex: 12,
  drawerSection: 'Operations',
})

const apiFetch = useApiFetch()
const notifications = useAdminNotifications()
const users = shallowRef<AdminUser[]>([])
const groups = shallowRef<AccessGroup[]>([])
const loadingOptions = shallowRef(false)
const resolvingAudience = shallowRef(false)
const renderingPreview = shallowRef(false)
const resolvedAudience = shallowRef<NotificationAudienceResult | null>(null)
const renderedPreview = shallowRef<NotificationPreview | null>(null)
const actionError = shallowRef('')

interface AdminOptionPage<T> {
  items: T[]
  total: number
}

async function loadAllOptionPages<T>(path: string): Promise<T[]> {
  const pageSize = 100
  const first = await apiFetch<AdminOptionPage<T>>(
    `${path}&page=1&pageSize=${pageSize}`,
  )
  const pageCount = Math.ceil(first.total / pageSize)
  if (pageCount <= 1) return first.items

  const remaining = await Promise.all(
    Array.from({ length: pageCount - 1 }, (_, index) =>
      apiFetch<AdminOptionPage<T>>(
        `${path}&page=${index + 2}&pageSize=${pageSize}`,
      ),
    ),
  )
  return [first, ...remaining].flatMap((page) => page.items)
}

onMounted(async () => {
  loadingOptions.value = true
  actionError.value = ''
  try {
    const [userItems, groupItems] = await Promise.all([
      loadAllOptionPages<AdminUser>(
        '/api/v1/admin/users?sortBy=preferredUsername&sortDir=asc',
      ),
      loadAllOptionPages<AccessGroup>(
        '/api/v1/admin/groups?sortBy=name&sortDir=asc',
      ),
    ])
    users.value = userItems
    groups.value = groupItems
  } catch (error) {
    actionError.value = toAdminNotificationErrorMessage(error)
  } finally {
    loadingOptions.value = false
  }
})

async function resolveAudience(payload: NotificationAudiencePayload) {
  actionError.value = ''
  resolvingAudience.value = true
  try {
    resolvedAudience.value = await notifications.resolveAudience(payload)
  } catch (error) {
    actionError.value = toAdminNotificationErrorMessage(error)
  } finally {
    resolvingAudience.value = false
  }
}

async function preview(
  payload: Pick<SendNotificationPayload, 'subject' | 'markdown'>,
) {
  actionError.value = ''
  renderingPreview.value = true
  try {
    renderedPreview.value = await notifications.preview(
      payload.subject,
      payload.markdown,
    )
  } catch (error) {
    actionError.value = toAdminNotificationErrorMessage(error)
  } finally {
    renderingPreview.value = false
  }
}

async function send(payload: SendNotificationPayload) {
  actionError.value = ''
  try {
    await notifications.send(payload)
    await navigateTo('/admin/notifications')
  } catch (error) {
    actionError.value = toAdminNotificationErrorMessage(error)
  }
}
</script>

<template>
  <v-container fluid class="app-page">
    <AppPageHeader
      icon="mdi-email-edit-outline"
      kicker="Communications"
      title="Send email"
      copy="Compose a sanitized message for selected users, access groups, every human account, or one external address."
      visual="cables"
      stat-label="Delivery mode"
      stat-value="Queued"
    />
    <v-alert
      v-if="actionError"
      class="mt-4"
      type="warning"
      variant="tonal"
      closable
      @click:close="actionError = ''"
    >
      {{ actionError }}
    </v-alert>
    <AdminEmailComposer
      class="mt-6"
      :audience="resolvedAudience"
      :groups="groups"
      :loading-options="loadingOptions"
      :preview="renderedPreview"
      :previewing="renderingPreview"
      :resolving="resolvingAudience"
      :saving="notifications.saving.value"
      :users="users"
      @invalidate:audience="resolvedAudience = null"
      @invalidate:preview="renderedPreview = null"
      @preview="preview"
      @resolve="resolveAudience"
      @send="send"
    />
  </v-container>
</template>
