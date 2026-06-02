<script setup lang="ts">
import type { MonitoringService } from '~/types/monitoring'
import { formatDateTime, formatDurationMs } from '~/utils/formatters'

const props = defineProps<{
  loading: boolean
  service: MonitoringService | null
}>()

const isOpen = defineModel<boolean>({ default: false })

const title = computed(() =>
  props.service ? `${displayName(props.service)} details` : 'Service details',
)
const subtitle = computed(() =>
  props.service
    ? props.service.url
    : 'Monitoring service check details and current status.',
)
const statusColor = computed(() => {
  if (!props.service?.enabled) {
    return 'grey'
  }
  return props.service.status === 'degraded' ? 'warning' : 'success'
})
const statusIcon = computed(() =>
  props.service?.status === 'degraded'
    ? 'mdi-alert-circle-outline'
    : 'mdi-check-circle-outline',
)
const statusLabel = computed(() => {
  if (!props.service?.enabled) {
    return 'Disabled'
  }
  return props.service.status === 'degraded' ? 'Degraded' : 'OK'
})
const reason = computed(() => {
  const service = props.service
  if (!service) {
    return ''
  }
  if (!service.enabled) {
    return 'Monitoring is disabled for this service.'
  }
  if (!service.lastCheckedAt) {
    return 'No check has completed yet.'
  }
  if (service.lastError) {
    return service.lastError
  }
  if (service.status === 'degraded') {
    return 'The latest check marked this service as degraded.'
  }
  return `The latest check matched HTTP ${service.expectedStatusCode}.`
})
const receivedStatusCode = computed(() => {
  const statusCode = props.service?.lastStatusCode
  return statusCode == null ? 'No HTTP response' : String(statusCode)
})
const failuresLabel = computed(() => {
  const failures = props.service?.consecutiveFailures ?? 0
  return failures === 1 ? '1 failure' : `${failures} failures`
})

function displayName(service: MonitoringService) {
  return service.displayName || service.name
}

function intervalLabel(seconds: number) {
  if (seconds < 60) {
    return `${seconds}s`
  }
  if (seconds % 3600 === 0) {
    return `${seconds / 3600}h`
  }
  if (seconds % 60 === 0) {
    return `${seconds / 60}m`
  }
  return `${seconds}s`
}
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    icon="mdi-information-outline"
    :icon-color="statusColor"
    :loading="props.loading"
    max-width="860"
    :subtitle="subtitle"
    :title="title"
  >
    <div
      v-if="props.service"
      class="admin-monitoring-service-details-dialog"
    >
      <section class="admin-monitoring-service-details-dialog__summary">
        <div class="admin-monitoring-service-details-dialog__status">
          <v-chip
            size="small"
            label
            variant="tonal"
            :color="statusColor"
            :prepend-icon="statusIcon"
          >
            {{ statusLabel }}
          </v-chip>
          <span class="admin-monitoring-service-details-dialog__failures">
            {{ failuresLabel }}
          </span>
        </div>

        <p class="admin-monitoring-service-details-dialog__reason">
          {{ reason }}
        </p>
      </section>

      <section class="admin-monitoring-service-details-dialog__section">
        <h2 class="admin-monitoring-service-details-dialog__section-title">
          Last check
        </h2>

        <dl class="admin-monitoring-service-details-dialog__grid">
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Checked at</dt>
            <dd>{{ formatDateTime(props.service.lastCheckedAt) }}</dd>
          </div>
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Latency</dt>
            <dd>
              {{
                props.service.lastCheckedAt
                  ? formatDurationMs(props.service.lastLatencyMs)
                  : 'Pending'
              }}
            </dd>
          </div>
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Expected HTTP</dt>
            <dd>{{ props.service.expectedStatusCode }}</dd>
          </div>
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Received HTTP</dt>
            <dd>{{ receivedStatusCode }}</dd>
          </div>
        </dl>
      </section>

      <section class="admin-monitoring-service-details-dialog__section">
        <h2 class="admin-monitoring-service-details-dialog__section-title">
          Error
        </h2>
        <p class="admin-monitoring-service-details-dialog__error">
          {{ props.service.lastError || 'No error recorded.' }}
        </p>
      </section>

      <section class="admin-monitoring-service-details-dialog__section">
        <h2 class="admin-monitoring-service-details-dialog__section-title">
          Configuration
        </h2>

        <dl class="admin-monitoring-service-details-dialog__grid">
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Name</dt>
            <dd>{{ props.service.name }}</dd>
          </div>
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Display name</dt>
            <dd>{{ props.service.displayName || 'No display name' }}</dd>
          </div>
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Interval</dt>
            <dd>{{ intervalLabel(props.service.intervalSeconds) }}</dd>
          </div>
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Enabled</dt>
            <dd>{{ props.service.enabled ? 'Yes' : 'No' }}</dd>
          </div>
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Created</dt>
            <dd>{{ formatDateTime(props.service.createdAt) }}</dd>
          </div>
          <div class="admin-monitoring-service-details-dialog__item">
            <dt>Updated</dt>
            <dd>{{ formatDateTime(props.service.updatedAt) }}</dd>
          </div>
        </dl>
      </section>
    </div>

    <template #actions>
      <v-spacer />
      <AppDialogCloseButton :disabled="props.loading" @click="isOpen = false" />
    </template>
  </AppDialogCard>
</template>

<style scoped>
.admin-monitoring-service-details-dialog {
  display: grid;
  gap: 16px;
}

.admin-monitoring-service-details-dialog__summary,
.admin-monitoring-service-details-dialog__section {
  display: grid;
  gap: 12px;
  padding: 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.5);
  border-radius: 8px;
}

.admin-monitoring-service-details-dialog__status {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.admin-monitoring-service-details-dialog__failures {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.875rem;
  font-weight: 600;
}

.admin-monitoring-service-details-dialog__reason,
.admin-monitoring-service-details-dialog__error {
  min-width: 0;
  margin: 0;
  color: rgb(var(--app-shell-text));
  overflow-wrap: anywhere;
}

.admin-monitoring-service-details-dialog__error {
  color: rgb(var(--app-shell-text-secondary));
}

.admin-monitoring-service-details-dialog__section-title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 700;
}

.admin-monitoring-service-details-dialog__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 0;
}

.admin-monitoring-service-details-dialog__item {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.admin-monitoring-service-details-dialog__item dt {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
}

.admin-monitoring-service-details-dialog__item dd {
  min-width: 0;
  margin: 0;
  color: rgb(var(--app-shell-text));
  overflow-wrap: anywhere;
}

@media (max-width: 640px) {
  .admin-monitoring-service-details-dialog__grid {
    grid-template-columns: 1fr;
  }
}
</style>
