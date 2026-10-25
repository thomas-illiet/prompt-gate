<script setup lang="ts">
import type {
  DashboardScope,
  DashboardToolUsage,
  DashboardTopToolsResponse,
  UsageWindow,
} from '~/types/user-service'
import { formatNumber } from '~/utils/formatters'

const props = defineProps<{
  refreshVersion: number
  scope: DashboardScope
  window: UsageWindow
}>()

const apiFetch = useApiFetch()
const data = shallowRef<DashboardTopToolsResponse | null>(null)
const loading = shallowRef(false)
const error = shallowRef<string | null>(null)
let controller: AbortController | null = null
let requestVersion = 0

const endpoint = computed(() =>
  props.scope === 'global'
    ? '/api/v1/admin/dashboard/top-tools'
    : '/api/v1/me/dashboard/top-tools',
)
const currentKey = computed(() => `${props.scope}:${props.window}`)
const rankedItems = computed(() =>
  [...(data.value?.items ?? [])].sort((left, right) => {
    if (right.calls !== left.calls) return right.calls - left.calls
    return left.name.localeCompare(right.name)
  }),
)
const leaderCalls = computed(() => rankedItems.value[0]?.calls ?? 0)
const coverageLabel = computed(() => {
  const response = data.value
  if (!response || response.totalCalls === 0) {
    return 'No observed tool calls for this period.'
  }

  const ratio = Math.min(
    1,
    Math.max(0, response.attributedCalls / response.totalCalls),
  )
  const percent = new Intl.NumberFormat('en-US', {
    maximumFractionDigits: 1,
    style: 'percent',
  }).format(ratio)

  return `${formatNumber(response.attributedCalls)} of ${formatNumber(
    response.totalCalls,
  )} observed calls attributed by name (${percent}).`
})
const emptyLabel = computed(() =>
  (data.value?.totalCalls ?? 0) > 0
    ? 'Observed tool calls could not be attributed by name for this period.'
    : 'No observed tool calls for this period.',
)
watch(
  [currentKey, () => props.refreshVersion],
  ([key], previous) => {
    if (!previous || key !== previous[0]) {
      data.value = null
      error.value = null
    }
    void load(key)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  requestVersion += 1
  controller?.abort()
})

async function load(requestedKey = currentKey.value) {
  const version = ++requestVersion
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = null

  try {
    const response = await apiFetch<DashboardTopToolsResponse>(
      `${endpoint.value}?window=${props.window}`,
      { signal: controller.signal },
    )
    if (version === requestVersion && requestedKey === currentKey.value) {
      data.value = response
    }
  } catch (fetchError) {
    if (
      version === requestVersion &&
      requestedKey === currentKey.value &&
      !isAbortError(fetchError)
    ) {
      error.value = data.value
        ? 'Unable to refresh top tool calls. Showing the last loaded data.'
        : 'Unable to load top tool calls.'
    }
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

function isAbortError(value: unknown) {
  return value instanceof Error && value.name === 'AbortError'
}

function barWidth(item: DashboardToolUsage) {
  if (leaderCalls.value <= 0) return '0%'
  return `${Math.max(2, (item.calls / leaderCalls.value) * 100)}%`
}

function callsLabel(calls: number) {
  return `${formatNumber(calls)} observed ${calls === 1 ? 'call' : 'calls'}`
}
</script>

<template>
  <AppSectionCard
    icon="mdi-tools"
    title="Top tool calls"
    subtitle="Most invoked tools across observed model responses"
  >
    <div class="dashboard-top-tools" :aria-busy="loading">
      <div
        v-if="loading && !data"
        class="dashboard-top-tools__loading"
        aria-label="Loading top tool calls"
      >
        <span v-for="index in 5" :key="index" />
      </div>

      <v-alert
        v-else-if="error && !data"
        type="warning"
        variant="tonal"
        rounded="lg"
      >
        <div class="dashboard-top-tools__alert">
          <span>{{ error }}</span>
          <v-btn
            size="small"
            variant="text"
            prepend-icon="mdi-refresh"
            @click="load()"
          >
            Retry
          </v-btn>
        </div>
      </v-alert>

      <template v-else-if="data">
        <v-alert v-if="error" type="warning" variant="tonal" rounded="lg">
          <div class="dashboard-top-tools__alert">
            <span>{{ error }}</span>
            <v-btn
              size="small"
              variant="text"
              prepend-icon="mdi-refresh"
              @click="load()"
            >
              Retry
            </v-btn>
          </div>
        </v-alert>

        <p class="dashboard-top-tools__coverage" aria-live="polite">
          {{ coverageLabel }}
        </p>

        <p v-if="rankedItems.length === 0" class="dashboard-top-tools__empty">
          {{ emptyLabel }}
        </p>

        <ol
          v-else
          class="dashboard-top-tools__list"
          aria-label="Top tools ranked by observed calls"
        >
          <li
            v-for="(item, index) in rankedItems"
            :key="item.name"
            class="dashboard-top-tools__item"
            data-test="tool-item"
          >
            <span class="dashboard-top-tools__rank" aria-hidden="true">
              {{ index + 1 }}
            </span>
            <div class="dashboard-top-tools__detail">
              <div class="dashboard-top-tools__label">
                <strong>{{ item.name }}</strong>
                <span>{{ callsLabel(item.calls) }}</span>
              </div>
              <div class="dashboard-top-tools__track" aria-hidden="true">
                <span :style="{ width: barWidth(item) }" />
              </div>
            </div>
          </li>
        </ol>
      </template>
    </div>
  </AppSectionCard>
</template>

<style scoped>
.dashboard-top-tools {
  display: grid;
  min-height: 220px;
  gap: 14px;
  padding: 8px 20px 20px;
}

.dashboard-top-tools__coverage,
.dashboard-top-tools__empty {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.88rem;
}

.dashboard-top-tools__empty {
  display: grid;
  min-height: 150px;
  place-items: center;
  text-align: center;
}

.dashboard-top-tools__loading {
  display: grid;
  align-content: center;
  gap: 15px;
  min-height: 190px;
}

.dashboard-top-tools__loading span {
  height: 22px;
  border-radius: 5px;
  background: rgba(var(--app-shell-border), 0.34);
}

.dashboard-top-tools__loading span:nth-child(2) {
  width: 84%;
}
.dashboard-top-tools__loading span:nth-child(3) {
  width: 70%;
}
.dashboard-top-tools__loading span:nth-child(4) {
  width: 56%;
}
.dashboard-top-tools__loading span:nth-child(5) {
  width: 42%;
}

.dashboard-top-tools__alert {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dashboard-top-tools__list {
  display: grid;
  gap: 14px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.dashboard-top-tools__item {
  display: grid;
  grid-template-columns: 30px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
}

.dashboard-top-tools__rank {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid rgba(var(--app-shell-border), 0.58);
  border-radius: 50%;
  color: rgb(var(--app-shell-text-secondary));
  font-family: var(--app-font-mono);
  font-size: 0.76rem;
  font-weight: 750;
}

.dashboard-top-tools__detail {
  display: grid;
  min-width: 0;
  gap: 7px;
}

.dashboard-top-tools__label {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.dashboard-top-tools__label strong {
  min-width: 0;
  overflow-wrap: anywhere;
}

.dashboard-top-tools__label span {
  flex: 0 0 auto;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
  font-weight: 650;
}

.dashboard-top-tools__track {
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(var(--app-shell-border), 0.32);
}

.dashboard-top-tools__track span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(
    90deg,
    rgb(var(--v-theme-primary)),
    rgba(var(--app-accent-coral), 0.92)
  );
}

@media (max-width: 599px) {
  .dashboard-top-tools {
    padding: 8px 12px 16px;
  }

  .dashboard-top-tools__label {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .dashboard-top-tools__alert {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
