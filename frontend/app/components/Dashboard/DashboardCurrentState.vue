<script setup lang="ts">
import type { CurrentQuotaStatus } from '~/types/subscriptions'
import { formatDateTime, formatNumber } from '~/utils/formatters'

const props = defineProps<{
  refreshVersion: number
}>()

interface QuotaWindowView {
  key: '5h' | '7d'
  label: string
  used: number
  limit: number | null
  remaining: number | null
  resetAt: string | null
}

const apiFetch = useApiFetch()
const quota = shallowRef<CurrentQuotaStatus | null>(null)
const loading = shallowRef(false)
const error = shallowRef<string | null>(null)
let controller: AbortController | null = null
let requestVersion = 0

const quotaWindows = computed<QuotaWindowView[]>(() => [
  {
    key: '5h',
    label: '5-hour window',
    used: quota.value?.used5hTokens ?? 0,
    limit: quota.value?.quota5hTokens ?? null,
    remaining: quota.value?.remaining5hTokens ?? null,
    resetAt: quota.value?.reset5hAt ?? null,
  },
  {
    key: '7d',
    label: '7-day window',
    used: quota.value?.used7dTokens ?? 0,
    limit: quota.value?.quota7dTokens ?? null,
    remaining: quota.value?.remaining7dTokens ?? null,
    resetAt: quota.value?.reset7dAt ?? null,
  },
])

async function loadCurrentState() {
  const version = ++requestVersion
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = null

  try {
    const response = await apiFetch<CurrentQuotaStatus>('/api/v1/me/quota', {
      signal: controller.signal,
    })
    if (version === requestVersion) {
      quota.value = response
    }
  } catch (fetchError) {
    if (
      version === requestVersion &&
      !(fetchError instanceof Error && fetchError.name === 'AbortError')
    ) {
      error.value = 'Unable to load your live quota.'
    }
  } finally {
    if (version === requestVersion) {
      loading.value = false
    }
  }
}

watch(
  () => props.refreshVersion,
  () => void loadCurrentState(),
  { immediate: true },
)

onBeforeUnmount(() => {
  requestVersion += 1
  controller?.abort()
})

function quotaPercent(window: QuotaWindowView) {
  if (window.limit == null || window.limit <= 0) {
    return null
  }
  return (window.used / window.limit) * 100
}

function quotaState(window: QuotaWindowView) {
  const percent = quotaPercent(window)
  if (percent == null) return 'unlimited' as const
  if (percent >= 100) return 'critical' as const
  if (percent >= 80) return 'warning' as const
  return 'neutral' as const
}

function quotaColor(window: QuotaWindowView) {
  const state = quotaState(window)
  if (state === 'critical') return 'error'
  if (state === 'warning') return 'warning'
  return 'primary'
}

function quotaStateLabel(window: QuotaWindowView) {
  const state = quotaState(window)
  if (state === 'critical') return 'Quota exhausted'
  if (state === 'warning') return 'Approaching limit'
  if (state === 'unlimited') return 'Unlimited'
  return 'Within limit'
}

function percentLabel(window: QuotaWindowView) {
  const percent = quotaPercent(window)
  if (percent == null) return 'Unlimited'
  return `${new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 }).format(percent)}% used`
}

function remainingLabel(window: QuotaWindowView) {
  if (window.limit == null) return 'Unlimited remaining'
  const remaining = window.remaining ?? Math.max(0, window.limit - window.used)
  return `${formatNumber(remaining)} tokens remaining`
}
</script>

<template>
  <AppSectionCard
    icon="mdi-gauge"
    title="Plan and live quota"
    subtitle="Current 5-hour and 7-day limits, independent of the selected period"
  >
    <template v-if="quota?.plan" #actions>
      <v-chip color="primary" label size="small" variant="tonal">
        {{ quota.plan.name }}
      </v-chip>
    </template>

    <div class="dashboard-current-state" :aria-busy="loading">
      <div v-if="loading && !quota" class="dashboard-current-state__loading">
        <span v-for="index in 3" :key="index" />
      </div>

      <v-alert v-else-if="error" type="warning" variant="tonal" rounded="lg">
        <div class="dashboard-current-state__alert">
          <span>{{ error }}</span>
          <v-btn
            size="small"
            variant="text"
            prepend-icon="mdi-refresh"
            @click="loadCurrentState"
          >
            Retry
          </v-btn>
        </div>
      </v-alert>

      <template v-else>
        <v-alert
          v-if="quota && !quota.hasSubscription"
          type="warning"
          variant="tonal"
          rounded="lg"
        >
          No subscription plan is currently assigned. Contact an administrator
          before sending requests.
        </v-alert>

        <div v-else class="dashboard-current-state__quota-grid">
          <article
            v-for="window in quotaWindows"
            :key="window.key"
            class="dashboard-current-state__quota"
            :class="`dashboard-current-state__quota--${quotaState(window)}`"
          >
            <div class="dashboard-current-state__quota-heading">
              <div>
                <strong>{{ window.label }}</strong>
                <span>{{ formatNumber(window.used) }} tokens used</span>
              </div>
              <v-chip
                :color="quotaColor(window)"
                label
                size="small"
                variant="tonal"
              >
                {{ quotaStateLabel(window) }}
              </v-chip>
            </div>

            <template v-if="quotaState(window) !== 'unlimited'">
              <div class="dashboard-current-state__quota-value">
                <strong>{{ percentLabel(window) }}</strong>
                <span>{{ remainingLabel(window) }}</span>
              </div>
              <v-progress-linear
                :aria-label="`${window.label}: ${percentLabel(window)}`"
                :color="quotaColor(window)"
                :model-value="Math.min(100, quotaPercent(window) ?? 0)"
                bg-color="surface-variant"
                height="8"
                rounded
              />
            </template>

            <div v-else class="dashboard-current-state__unlimited">
              <v-icon icon="mdi-infinity" size="24" />
              <strong>Unlimited</strong>
              <span>No token ceiling for this window</span>
            </div>

            <span class="dashboard-current-state__reset">
              {{
                window.resetAt
                  ? `Resets ${formatDateTime(window.resetAt)}`
                  : 'No reset scheduled'
              }}
            </span>
          </article>
        </div>
      </template>
    </div>
  </AppSectionCard>
</template>

<style scoped>
.dashboard-current-state {
  display: grid;
  gap: 18px;
  padding: 20px 24px 24px;
}

.dashboard-current-state__loading,
.dashboard-current-state__quota-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.dashboard-current-state__loading span {
  min-height: 108px;
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-border), 0.32);
}

.dashboard-current-state__quota {
  display: grid;
  gap: 13px;
  padding: 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.5);
}

.dashboard-current-state__quota--warning {
  border-color: rgba(var(--v-theme-warning), 0.48);
}

.dashboard-current-state__quota--critical {
  border-color: rgba(var(--v-theme-error), 0.55);
}

.dashboard-current-state__quota-heading,
.dashboard-current-state__quota-value,
.dashboard-current-state__alert {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dashboard-current-state__quota-heading > div {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.dashboard-current-state__quota-heading span,
.dashboard-current-state__quota-value span,
.dashboard-current-state__reset {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
}

.dashboard-current-state__quota-value strong {
  font-size: 1.15rem;
}

.dashboard-current-state__unlimited {
  display: flex;
  align-items: center;
  gap: 8px;
  color: rgb(var(--app-shell-text-secondary));
}

.dashboard-current-state__unlimited strong {
  color: rgb(var(--app-shell-text-primary));
}

.dashboard-current-state__unlimited span {
  font-size: 0.86rem;
}

@media (max-width: 599px) {
  .dashboard-current-state {
    padding: 16px 16px 20px;
  }

  .dashboard-current-state__loading,
  .dashboard-current-state__quota-grid {
    grid-template-columns: 1fr;
  }

  .dashboard-current-state__quota-heading,
  .dashboard-current-state__quota-value,
  .dashboard-current-state__alert {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
