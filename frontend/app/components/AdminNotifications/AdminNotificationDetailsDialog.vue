<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type {
  NotificationDelivery,
  NotificationDeliveryStatus,
  NotificationMessageDetail,
} from '~/types/notifications'
import { formatDateTime } from '~/utils/formatters'

type DeliveryFilter = NotificationDeliveryStatus | 'all'

const props = defineProps<{
  loading: boolean
  message: NotificationMessageDetail | null
  retrying: boolean
}>()

const isOpen = defineModel<boolean>({ required: true })
const emit = defineEmits<{ retry: [id: string] }>()
const activeTab = shallowRef<'message' | 'deliveries'>('message')
const deliveryFilter = shallowRef<DeliveryFilter>('all')

const headers: DataTableHeader[] = [
  { title: 'Recipient', key: 'recipientEmail' },
  { title: 'Status', key: 'status' },
  { title: 'Attempts', key: 'attempts', align: 'center' },
  { title: 'Last activity', key: 'lastAttemptAt' },
  { title: 'Details', key: 'lastError', sortable: false },
]

const counts = computed(() => {
  const deliveries = props.message?.deliveries ?? []
  return {
    all: deliveries.length,
    failed: deliveries.filter((item) => item.status === 'failed').length,
    pending: deliveries.filter((item) => item.status === 'pending').length,
    processing: deliveries.filter((item) => item.status === 'processing')
      .length,
    sent: deliveries.filter((item) => item.status === 'sent').length,
  }
})
const unresolvedCount = computed(
  () => counts.value.failed + counts.value.pending + counts.value.processing,
)
const title = computed(() => props.message?.subject ?? 'Email delivery')
const subtitle = computed(() => {
  const message = props.message
  if (!message) return 'Message content and delivery progress.'
  return `${formatDateTime(message.createdAt)} · ${message.recipients} recipient${message.recipients === 1 ? '' : 's'}`
})
const iconColor = computed(() => {
  if (counts.value.failed > 0) return 'error'
  if (counts.value.pending + counts.value.processing > 0) return 'warning'
  return 'success'
})
const filters = computed(() => [
  { label: 'All', value: 'all' as const, count: counts.value.all },
  { label: 'Sent', value: 'sent' as const, count: counts.value.sent },
  {
    label: 'In progress',
    value: 'pending' as const,
    count: counts.value.pending + counts.value.processing,
  },
  { label: 'Failed', value: 'failed' as const, count: counts.value.failed },
])
const visibleDeliveries = computed(() => {
  const rank: Record<NotificationDeliveryStatus, number> = {
    failed: 0,
    processing: 1,
    pending: 2,
    sent: 3,
  }
  const deliveries = props.message?.deliveries ?? []
  const filtered = deliveries.filter((delivery) => {
    if (deliveryFilter.value === 'all') return true
    if (deliveryFilter.value === 'pending') {
      return delivery.status === 'pending' || delivery.status === 'processing'
    }
    return delivery.status === deliveryFilter.value
  })
  return [...filtered].sort(
    (left, right) => rank[left.status] - rank[right.status],
  )
})

watch(
  () => props.message?.id,
  () => {
    deliveryFilter.value = 'all'
    activeTab.value = unresolvedCount.value > 0 ? 'deliveries' : 'message'
  },
  { immediate: true },
)

function statusColor(status: NotificationDeliveryStatus) {
  if (status === 'failed') return 'error'
  if (status === 'sent') return 'success'
  return 'warning'
}

function deliveryDetails(delivery: NotificationDelivery) {
  if (delivery.lastError) return delivery.lastError
  if (delivery.status === 'pending' && delivery.nextAttemptAt) {
    return `Next attempt ${formatDateTime(delivery.nextAttemptAt)}`
  }
  if (delivery.status === 'processing') return 'Delivery in progress'
  return '—'
}
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    content-class="admin-notification-details__content"
    icon="mdi-email-fast-outline"
    :icon-color="iconColor"
    :loading="props.loading || props.retrying"
    max-width="980"
    :subtitle="subtitle"
    :title="title"
  >
    <v-skeleton-loader v-if="props.loading" type="article, table" />

    <div v-else-if="props.message" class="admin-notification-details">
      <section
        class="admin-notification-details__summary"
        aria-label="Delivery summary"
      >
        <div class="admin-notification-details__identity">
          <span>Type</span>
          <v-chip size="small" variant="tonal">
            {{ props.message.kind.replaceAll('_', ' ') }}
          </v-chip>
        </div>
        <div class="admin-notification-details__metric">
          <span>Recipients</span>
          <strong>{{ counts.all }}</strong>
        </div>
        <div
          class="admin-notification-details__metric admin-notification-details__metric--success"
        >
          <span>Sent</span>
          <strong>{{ counts.sent }}</strong>
        </div>
        <div
          class="admin-notification-details__metric admin-notification-details__metric--warning"
        >
          <span>In progress</span>
          <strong>{{ counts.pending + counts.processing }}</strong>
        </div>
        <div
          class="admin-notification-details__metric admin-notification-details__metric--error"
        >
          <span>Failed</span>
          <strong>{{ counts.failed }}</strong>
        </div>
      </section>

      <v-tabs
        v-model="activeTab"
        class="admin-notification-details__tabs"
        color="primary"
      >
        <v-tab prepend-icon="mdi-email-open-outline" value="message">
          Message
        </v-tab>
        <v-tab prepend-icon="mdi-truck-delivery-outline" value="deliveries">
          Delivery
          <v-badge
            v-if="unresolvedCount"
            color="error"
            :content="unresolvedCount"
            inline
          />
        </v-tab>
      </v-tabs>

      <v-window v-model="activeTab">
        <v-window-item value="message">
          <section
            class="admin-notification-details__panel"
            aria-labelledby="notification-preview-heading"
          >
            <div class="admin-notification-details__panel-heading">
              <div>
                <h2 id="notification-preview-heading">Recipient preview</h2>
                <p>This is the sanitized content stored for delivery.</p>
              </div>
              <v-chip
                prepend-icon="mdi-shield-check-outline"
                size="small"
                variant="tonal"
              >
                Sanitized
              </v-chip>
            </div>
            <iframe
              class="admin-notification-details__message"
              :srcdoc="props.message.htmlBody"
              title="Stored email preview"
              sandbox=""
            />
          </section>
        </v-window-item>

        <v-window-item value="deliveries">
          <section
            class="admin-notification-details__panel"
            aria-labelledby="notification-deliveries-heading"
          >
            <div
              class="admin-notification-details__panel-heading admin-notification-details__panel-heading--deliveries"
            >
              <div>
                <h2 id="notification-deliveries-heading">Recipient delivery</h2>
                <p>Failures and active retries are shown first.</p>
              </div>
              <v-btn-toggle
                v-model="deliveryFilter"
                color="primary"
                density="comfortable"
                mandatory
                variant="outlined"
              >
                <v-btn
                  v-for="filter in filters"
                  :key="filter.value"
                  size="small"
                  :value="filter.value"
                >
                  {{ filter.label }} · {{ filter.count }}
                </v-btn>
              </v-btn-toggle>
            </div>

            <v-data-table
              class="admin-notification-details__table"
              density="comfortable"
              :headers="headers"
              :items="visibleDeliveries"
              :items-per-page="10"
            >
              <template #item.recipientEmail="{ item }">
                <div class="admin-notification-details__recipient">
                  <strong v-if="item.recipientName">
                    {{ item.recipientName }}
                  </strong>
                  <span>{{ item.recipientEmail }}</span>
                </div>
              </template>
              <template #item.status="{ item }">
                <v-chip
                  :color="statusColor(item.status)"
                  label
                  size="small"
                  variant="tonal"
                >
                  {{ item.status }}
                </v-chip>
              </template>
              <template #item.attempts="{ item }">
                <div class="text-center">{{ item.attempts }}</div>
              </template>
              <template #item.lastAttemptAt="{ item }">
                {{
                  item.lastAttemptAt
                    ? formatDateTime(item.lastAttemptAt)
                    : 'Not attempted'
                }}
              </template>
              <template #item.lastError="{ item }">
                <span :class="{ 'text-error': item.lastError }">
                  {{ deliveryDetails(item) }}
                </span>
              </template>
              <template #no-data>
                <div class="admin-notification-details__empty">
                  No deliveries match this filter.
                </div>
              </template>
            </v-data-table>
          </section>
        </v-window-item>
      </v-window>
    </div>

    <template #actions>
      <AppDialogCloseButton
        :disabled="props.loading || props.retrying"
        @click="isOpen = false"
      />
      <AppDialogActionButton
        v-if="props.message && counts.failed > 0"
        color="primary"
        prepend-icon="mdi-reload"
        :label="`Retry ${counts.failed} failed`"
        :loading="props.retrying"
        @click="emit('retry', props.message.id)"
      />
    </template>
  </AppDialogCard>
</template>

<style scoped>
:deep(.admin-notification-details__content) {
  padding: 0;
}

.admin-notification-details {
  display: grid;
}

.admin-notification-details__summary {
  display: grid;
  grid-template-columns: minmax(160px, 1.5fr) repeat(4, minmax(96px, 1fr));
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.38);
  background: rgba(var(--app-shell-surface-muted), 0.36);
}

.admin-notification-details__identity,
.admin-notification-details__metric {
  display: grid;
  gap: 5px;
  padding: 16px 18px;
  border-right: 1px solid rgba(var(--app-shell-border), 0.32);
}

.admin-notification-details__identity span,
.admin-notification-details__metric span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.admin-notification-details__metric strong {
  font-size: 1.35rem;
  line-height: 1;
}

.admin-notification-details__metric--success strong {
  color: rgb(var(--v-theme-success));
}

.admin-notification-details__metric--warning strong {
  color: rgb(var(--v-theme-warning));
}

.admin-notification-details__metric--error strong {
  color: rgb(var(--v-theme-error));
}

.admin-notification-details__tabs {
  margin-top: 20px;
  padding-inline: 12px;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.36);
}

.admin-notification-details__panel {
  display: grid;
  gap: 16px;
  padding: 20px 24px 24px;
}

.admin-notification-details__panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.admin-notification-details__panel-heading h2,
.admin-notification-details__panel-heading p {
  margin: 0;
}

.admin-notification-details__panel-heading h2 {
  font-size: 1rem;
  font-weight: 800;
}

.admin-notification-details__panel-heading p {
  margin-top: 3px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.85rem;
}

.admin-notification-details__message {
  display: block;
  width: 100%;
  min-height: 720px;
  padding: 0;
  border: 1px solid rgba(var(--app-shell-border), 0.48);
  border-radius: var(--app-card-radius);
  background: white;
}

.admin-notification-details__table {
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.48);
  border-radius: var(--app-card-radius);
}

.admin-notification-details__recipient {
  display: grid;
  min-width: 170px;
  gap: 2px;
}

.admin-notification-details__recipient span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.8rem;
}

.admin-notification-details__empty {
  padding: 28px;
  color: rgb(var(--app-shell-text-secondary));
  text-align: center;
}

@media (max-width: 800px) {
  .admin-notification-details__summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .admin-notification-details__identity {
    grid-column: 1 / -1;
  }

  .admin-notification-details__panel-heading--deliveries {
    align-items: stretch;
    flex-direction: column;
  }
}

@media (max-width: 600px) {
  .admin-notification-details__panel {
    padding: 16px;
  }

  .admin-notification-details__panel-heading {
    align-items: flex-start;
  }
}
</style>
