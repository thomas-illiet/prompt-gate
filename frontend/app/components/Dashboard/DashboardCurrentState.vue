<script setup lang="ts">
import type { CurrentQuotaStatus } from '~/types/subscriptions'
import type {
  DashboardAdoption,
  DashboardQuotaRiskItem,
  DashboardQuotaRiskResponse,
  DashboardScope,
} from '~/types/user-service'
import {
  formatCompactNumber,
  formatDateTime,
  formatNumber,
} from '~/utils/formatters'

const props = defineProps<{
  adoption?: DashboardAdoption
  refreshVersion: number
  scope: DashboardScope
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
const risk = shallowRef<DashboardQuotaRiskResponse | null>(null)
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
  const requestedScope = props.scope
  const version = ++requestVersion
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = null

  if (requestedScope === 'global') {
    quota.value = null
  } else {
    risk.value = null
  }

  try {
    if (requestedScope === 'global') {
      const response = await apiFetch<DashboardQuotaRiskResponse>(
        '/api/v1/admin/dashboard/quota-risk',
        { signal: controller.signal },
      )
      if (version === requestVersion && props.scope === requestedScope) {
        risk.value = response
      }
    } else {
      const response = await apiFetch<CurrentQuotaStatus>('/api/v1/me/quota', {
        signal: controller.signal,
      })
      if (version === requestVersion && props.scope === requestedScope) {
        quota.value = response
      }
    }
  } catch (fetchError) {
    if (
      version === requestVersion &&
      !(fetchError instanceof Error && fetchError.name === 'AbortError')
    ) {
      error.value =
        requestedScope === 'global'
          ? 'Unable to load quota risk.'
          : 'Unable to load your live quota.'
    }
  } finally {
    if (version === requestVersion) {
      loading.value = false
    }
  }
}

watch(
  [() => props.scope, () => props.refreshVersion],
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

function riskAccountLink(item: DashboardQuotaRiskItem) {
  return {
    path:
      item.accountType === 'service'
        ? '/admin/service-accounts'
        : '/admin/users',
    query: { search: item.accountId },
  }
}

function riskStatusLabel(item: DashboardQuotaRiskItem) {
  return item.status === 'critical' ? 'Critical' : 'Warning'
}
</script>

<template>
  <AppSectionCard
    :icon="props.scope === 'global' ? 'mdi-radar' : 'mdi-gauge'"
    :title="props.scope === 'global' ? 'Needs attention' : 'Plan and live quota'"
    :subtitle="
      props.scope === 'global'
        ? 'Adoption and quota saturation across people and service accounts'
        : 'Current 5-hour and 7-day limits, independent of the selected period'
    "
  >
    <template v-if="props.scope === 'self' && quota?.plan" #actions>
      <v-chip color="primary" label size="small" variant="tonal">
        {{ quota.plan.name }}
      </v-chip>
    </template>

    <div class="dashboard-current-state" :aria-busy="loading">
      <div v-if="loading && !quota && !risk" class="dashboard-current-state__loading">
        <span v-for="index in 3" :key="index" />
      </div>

      <v-alert v-else-if="error" type="warning" variant="tonal" rounded="lg">
        <div class="dashboard-current-state__alert">
          <span>{{ error }}</span>
          <v-btn size="small" variant="text" prepend-icon="mdi-refresh" @click="loadCurrentState">
            Retry
          </v-btn>
        </div>
      </v-alert>

      <template v-else-if="props.scope === 'self'">
        <v-alert
          v-if="quota && !quota.hasSubscription"
          type="warning"
          variant="tonal"
          rounded="lg"
        >
          No subscription plan is currently assigned. Contact an administrator before sending requests.
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
              <v-chip :color="quotaColor(window)" label size="small" variant="tonal">
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
              {{ window.resetAt ? `Resets ${formatDateTime(window.resetAt)}` : 'No reset scheduled' }}
            </span>
          </article>
        </div>
      </template>

      <template v-else>
        <div class="dashboard-current-state__adoption">
          <article>
            <span>Active people</span>
            <strong>{{ formatCompactNumber(props.adoption?.activeUsers ?? 0) }}</strong>
          </article>
          <article>
            <span>Active service accounts</span>
            <strong>{{ formatCompactNumber(props.adoption?.activeServiceAccounts ?? 0) }}</strong>
          </article>
          <article>
            <span>Active virtual keys</span>
            <strong>{{ formatCompactNumber(props.adoption?.activeVirtualKeys ?? 0) }}</strong>
          </article>
          <article class="dashboard-current-state__risk-stat">
            <span>At risk / exhausted</span>
            <strong>{{ formatNumber(risk?.atRiskCount ?? 0) }} / {{ formatNumber(risk?.exhaustedCount ?? 0) }}</strong>
          </article>
        </div>

        <div class="dashboard-current-state__risk-header">
          <div>
            <strong>Most saturated accounts</strong>
            <span>Worst live window; unlimited quotas excluded</span>
          </div>
          <span v-if="risk?.projectionSyncedAt">
            Snapshot {{ formatDateTime(risk.projectionSyncedAt) }}
          </span>
        </div>

        <div v-if="risk?.items.length" class="dashboard-current-state__risk-list">
          <NuxtLink
            v-for="item in risk.items"
            :key="item.accountId"
            class="dashboard-current-state__risk-row"
            :to="riskAccountLink(item)"
          >
            <div class="dashboard-current-state__risk-identity">
              <v-icon :icon="item.accountType === 'service' ? 'mdi-robot-outline' : 'mdi-account-outline'" size="20" />
              <div>
                <strong>{{ item.displayName }}</strong>
                <span>{{ item.planName }} · {{ item.window }}</span>
              </div>
            </div>
            <div class="dashboard-current-state__risk-reading">
              <v-chip
                class="dashboard-current-state__risk-chip"
                :color="item.status === 'critical' ? 'error' : 'warning'"
                label
                size="small"
                variant="tonal"
              >
                {{ riskStatusLabel(item) }} ·
                {{ new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 }).format(item.percent) }}%
              </v-chip>
              <div class="dashboard-current-state__risk-meta">
                <span>{{ formatNumber(item.remainingTokens) }} tokens left</span>
                <span>
                  {{ item.resetAt ? `Resets ${formatDateTime(item.resetAt)}` : 'No reset scheduled' }}
                </span>
              </div>
              <v-icon
                class="dashboard-current-state__risk-arrow"
                icon="mdi-chevron-right"
                size="18"
              />
            </div>
          </NuxtLink>
        </div>
        <p v-else class="dashboard-current-state__no-risk">
          No account is using 80% or more of a limited quota.
        </p>
      </template>
    </div>
  </AppSectionCard>
</template>

<style scoped>
.dashboard-current-state {
  display: grid;
  gap: 18px;
  padding: 4px 24px 24px;
}

.dashboard-current-state__loading,
.dashboard-current-state__quota-grid,
.dashboard-current-state__adoption {
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
.dashboard-current-state__alert,
.dashboard-current-state__risk-header,
.dashboard-current-state__risk-row,
.dashboard-current-state__risk-reading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dashboard-current-state__quota-heading > div,
.dashboard-current-state__risk-header > div,
.dashboard-current-state__risk-identity > div {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.dashboard-current-state__risk-meta {
  display: grid;
  gap: 2px;
  min-width: 0;
  text-align: right;
}

.dashboard-current-state__risk-chip {
  flex: 0 0 auto;
  flex-shrink: 0;
  max-width: none;
}

.dashboard-current-state__risk-chip :deep(.v-chip__content) {
  overflow: visible;
  text-overflow: clip;
}

.dashboard-current-state__quota-heading span,
.dashboard-current-state__quota-value span,
.dashboard-current-state__reset,
.dashboard-current-state__risk-header span,
.dashboard-current-state__risk-identity span,
.dashboard-current-state__risk-reading span,
.dashboard-current-state__no-risk {
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

.dashboard-current-state__adoption {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.dashboard-current-state__adoption article {
  display: grid;
  gap: 7px;
  padding: 14px;
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.5);
}

.dashboard-current-state__adoption span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.78rem;
  font-weight: 700;
  text-transform: uppercase;
}

.dashboard-current-state__adoption strong {
  font-size: 1.35rem;
}

.dashboard-current-state__risk-list {
  display: grid;
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  border-radius: var(--app-card-radius);
}

.dashboard-current-state__risk-row {
  padding: 12px 14px;
  color: inherit;
  text-decoration: none;
}

.dashboard-current-state__risk-row + .dashboard-current-state__risk-row {
  border-top: 1px solid rgba(var(--app-shell-border), 0.4);
}

.dashboard-current-state__risk-row:hover,
.dashboard-current-state__risk-row:focus-visible {
  background: rgba(var(--v-theme-primary), 0.07);
}

.dashboard-current-state__risk-identity {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.dashboard-current-state__no-risk {
  margin: 0;
  padding: 16px;
  border: 1px dashed rgba(var(--app-shell-border), 0.62);
  border-radius: var(--app-card-radius);
  text-align: center;
}

@media (max-width: 960px) {
  .dashboard-current-state__adoption {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-current-state__risk-reading {
    display: grid;
    flex: 0 0 auto;
    grid-template-columns: max-content max-content;
    justify-content: end;
  }

  .dashboard-current-state__risk-chip {
    grid-column: 1;
    grid-row: 1;
  }

  .dashboard-current-state__risk-arrow {
    grid-column: 2;
    grid-row: 1;
  }

  .dashboard-current-state__risk-meta {
    grid-column: 1 / -1;
    grid-row: 2;
  }
}

@media (max-width: 599px) {
  .dashboard-current-state {
    padding: 0 16px 20px;
  }

  .dashboard-current-state__loading,
  .dashboard-current-state__quota-grid,
  .dashboard-current-state__adoption {
    grid-template-columns: 1fr;
  }

  .dashboard-current-state__quota-heading,
  .dashboard-current-state__quota-value,
  .dashboard-current-state__alert,
  .dashboard-current-state__risk-header,
  .dashboard-current-state__risk-row {
    align-items: flex-start;
    flex-direction: column;
  }

  .dashboard-current-state__risk-reading {
    justify-content: start;
    width: 100%;
  }

  .dashboard-current-state__risk-meta {
    text-align: left;
  }
}
</style>
