<script setup lang="ts">
import type { FirewallSimulationResponse } from '~/types/firewall'

type SimulationResult =
  | { status: 'empty' }
  | { status: 'invalid' }
  | { status: 'pending' }
  | { response: FirewallSimulationResponse; status: 'allowed' | 'denied' }

const props = defineProps<{
  simulate: (clientIp: string) => Promise<FirewallSimulationResponse>
}>()

const clientIp = shallowRef('')
const simulationResponse = shallowRef<FirewallSimulationResponse | null>(null)
const simulationError = shallowRef('')
const debounceTimer = shallowRef<ReturnType<typeof setTimeout> | null>(null)
const waitingForDebounce = shallowRef(false)
const requestPending = shallowRef(false)
const requestVersion = shallowRef(0)
const isChecking = computed(
  () => waitingForDebounce.value || requestPending.value,
)
const simulation = computed<SimulationResult>(() => {
  if (!clientIp.value.trim()) {
    return { status: 'empty' }
  }

  if (isChecking.value) {
    return { status: 'pending' }
  }

  if (simulationError.value) {
    return { status: 'invalid' }
  }

  if (!simulationResponse.value) {
    return { status: 'empty' }
  }

  return simulationResponse.value.allowed
    ? { response: simulationResponse.value, status: 'allowed' }
    : { response: simulationResponse.value, status: 'denied' }
})
const resultColor = computed(() => {
  switch (simulation.value.status) {
    case 'allowed':
      return 'success'
    case 'denied':
      return 'error'
    case 'invalid':
      return 'warning'
    case 'pending':
      return 'primary'
    default:
      return 'grey'
  }
})
const resultClass = computed(
  () => `admin-firewall-simulator__result--${simulation.value.status}`,
)
const resultIcon = computed(() => {
  switch (simulation.value.status) {
    case 'allowed':
      return 'mdi-check-circle-outline'
    case 'denied':
      return 'mdi-close-octagon-outline'
    case 'invalid':
      return 'mdi-alert-circle-outline'
    case 'pending':
      return 'mdi-radar'
    default:
      return 'mdi-radar'
  }
})
const resultLabel = computed(() => {
  switch (simulation.value.status) {
    case 'allowed':
      return 'Allowed'
    case 'denied':
      return 'Denied'
    case 'invalid':
      return 'Invalid IP'
    case 'pending':
      return 'Checking'
    default:
      return 'Ready'
  }
})
const resultDescription = computed(() => {
  const result = simulation.value

  switch (result.status) {
    case 'allowed':
      return result.response.matchedRule
        ? 'Matched allow rule.'
        : 'No enabled rule matched. Traffic is allowed by default.'
    case 'denied':
      return 'Matched deny rule.'
    case 'invalid':
      return simulationError.value || 'Enter a valid IPv4 address.'
    case 'pending':
      return waitingForDebounce.value
        ? 'Waiting for typing to pause.'
        : 'Checking backend firewall rules.'
    default:
      return 'Enter an IPv4 address to simulate evaluation.'
  }
})
const resultDetail = computed(() => {
  const result = simulation.value

  switch (result.status) {
    case 'allowed':
    case 'denied':
      if (result.response.matchedRule) {
        return `${result.response.matchedRule.address} -> ${result.response.matchedRule.action}`
      }

      return 'No matching rule -> default allow'
    case 'invalid':
      return 'Simulation rejected by backend'
    case 'pending':
      return 'Waiting for backend result'
    default:
      return 'Waiting for IP'
  }
})
const matchedRule = computed(() =>
  simulation.value.status === 'allowed' || simulation.value.status === 'denied'
    ? simulation.value.response.matchedRule
    : null,
)

watch(clientIp, () => {
  scheduleSimulation()
})

onBeforeUnmount(() => {
  clearDebounceTimer()
  requestVersion.value += 1
})

// clearDebounceTimer cancels any pending firewall simulation.
function clearDebounceTimer() {
  if (!debounceTimer.value) {
    return
  }

  clearTimeout(debounceTimer.value)
  debounceTimer.value = null
}

// scheduleSimulation debounces simulation while the IP input changes.
function scheduleSimulation() {
  requestVersion.value += 1
  clearDebounceTimer()
  simulationResponse.value = null
  simulationError.value = ''

  if (!clientIp.value.trim()) {
    waitingForDebounce.value = false
    return
  }

  waitingForDebounce.value = true
  debounceTimer.value = setTimeout(() => {
    waitingForDebounce.value = false
    void runSimulation(requestVersion.value)
  }, 450)
}

// runImmediateSimulation cancels debounce and starts simulation immediately.
function runImmediateSimulation() {
  requestVersion.value += 1
  clearDebounceTimer()
  waitingForDebounce.value = false
  void runSimulation(requestVersion.value)
}

// runSimulation asks the backend which firewall rule matches the client IP.
async function runSimulation(version = requestVersion.value) {
  const ip = clientIp.value.trim()

  if (!ip) {
    simulationResponse.value = null
    simulationError.value = ''
    return
  }

  requestPending.value = true

  try {
    const response = await props.simulate(ip)
    if (version !== requestVersion.value) {
      return
    }

    simulationResponse.value = response
    simulationError.value = ''
  } catch (error) {
    if (version !== requestVersion.value) {
      return
    }

    simulationResponse.value = null
    simulationError.value =
      error instanceof Error ? error.message : 'Simulation failed.'
  } finally {
    if (version === requestVersion.value) {
      requestPending.value = false
    }
  }
}
</script>

<template>
  <div class="admin-firewall-simulator">
    <div class="admin-firewall-simulator__input-panel">
      <div class="admin-firewall-simulator__panel-heading">
        <v-avatar color="primary" variant="tonal" size="36">
          <v-icon icon="mdi-ip-network-outline" />
        </v-avatar>
        <div>
          <h2>Client IP</h2>
          <p>IPv4 address evaluated against enabled rules.</p>
        </div>
      </div>

      <v-text-field
        v-model="clientIp"
        label="IPv4 address"
        placeholder="192.168.1.42"
        density="comfortable"
        variant="outlined"
        flat
        clearable
        persistent-clear
        hide-details
        :error="simulation.status === 'invalid'"
        :loading="isChecking ? 'primary' : false"
        @keydown.enter.prevent="runImmediateSimulation"
      >
        <template #append-inner>
          <v-btn
            aria-label="Run firewall simulation"
            icon="mdi-radar"
            variant="text"
            size="small"
            :disabled="!clientIp.trim()"
            :loading="requestPending"
            @click="runImmediateSimulation"
          />
        </template>
      </v-text-field>
    </div>

    <div class="admin-firewall-simulator__result" :class="resultClass">
      <div class="admin-firewall-simulator__result-main">
        <v-avatar :color="resultColor" variant="flat" size="52">
          <v-progress-circular
            v-if="isChecking"
            indeterminate
            size="22"
            width="2"
          />
          <v-icon v-else :icon="resultIcon" size="28" />
        </v-avatar>

        <div class="admin-firewall-simulator__summary">
          <div class="admin-firewall-simulator__title">
            <strong>{{ resultLabel }}</strong>
            <v-chip
              v-if="matchedRule"
              size="small"
              label
              variant="flat"
              :color="matchedRule.action === 'allow' ? 'success' : 'error'"
            >
              Priority {{ matchedRule.priority }}
            </v-chip>
          </div>
          <p>{{ resultDescription }}</p>
          <small>{{ resultDetail }}</small>
        </div>
      </div>

      <div v-if="matchedRule" class="admin-firewall-simulator__rule">
        <div>
          <span>Rule</span>
          <strong>{{ matchedRule.address }}</strong>
        </div>
        <div>
          <span>Action</span>
          <strong>{{ matchedRule.action }}</strong>
        </div>
        <div>
          <span>Description</span>
          <strong>{{ matchedRule.description || 'No description' }}</strong>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.admin-firewall-simulator {
  display: grid;
  gap: 16px;
}

.admin-firewall-simulator__input-panel,
.admin-firewall-simulator__result {
  border: 1px solid rgba(var(--app-shell-border), 0.7);
  border-radius: 8px;
  background: rgb(var(--app-shell-surface-strong));
}

.admin-firewall-simulator__input-panel {
  display: grid;
  gap: 18px;
  min-width: 0;
  padding: 18px;
}

.admin-firewall-simulator__panel-heading {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.admin-firewall-simulator__panel-heading h2 {
  margin: 0;
  font-size: 1rem;
  font-weight: 750;
}

.admin-firewall-simulator__panel-heading p {
  margin: 2px 0 0;
  color: rgb(var(--app-shell-text-secondary));
}

.admin-firewall-simulator__result {
  display: grid;
  gap: 16px;
  min-width: 0;
  padding: 18px;
}

.admin-firewall-simulator__result--allowed {
  border-color: rgba(var(--v-theme-success), 0.55);
  background: rgba(var(--v-theme-success), 0.09);
}

.admin-firewall-simulator__result--denied {
  border-color: rgba(var(--v-theme-error), 0.6);
  background: rgba(var(--v-theme-error), 0.1);
}

.admin-firewall-simulator__result--invalid {
  border-color: rgba(var(--v-theme-warning), 0.62);
  background: rgba(var(--v-theme-warning), 0.1);
}

.admin-firewall-simulator__result--pending {
  border-color: rgba(var(--v-theme-primary), 0.55);
  background: rgba(var(--v-theme-primary), 0.09);
}

.admin-firewall-simulator__result-main {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
}

.admin-firewall-simulator__summary {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.admin-firewall-simulator__summary p,
.admin-firewall-simulator__summary small {
  display: block;
  margin: 0;
  color: rgb(var(--app-shell-text-primary));
}

.admin-firewall-simulator__summary small {
  color: rgb(var(--app-shell-text-secondary));
  font-weight: 600;
}

.admin-firewall-simulator__title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.admin-firewall-simulator__title strong {
  color: rgb(var(--app-shell-text-primary));
  font-size: 1.1rem;
}

.admin-firewall-simulator__rule {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
  padding: 12px;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  border-radius: 8px;
  background: rgba(var(--app-shell-surface), 0.72);
}

.admin-firewall-simulator__rule div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.admin-firewall-simulator__rule span {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.74rem;
  font-weight: 700;
  text-transform: uppercase;
}

.admin-firewall-simulator__rule strong {
  overflow: hidden;
  color: rgb(var(--app-shell-text-primary));
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (min-width: 960px) {
  .admin-firewall-simulator {
    grid-template-columns: minmax(280px, 0.8fr) minmax(0, 1.2fr);
    align-items: stretch;
  }

  .admin-firewall-simulator__rule {
    grid-template-columns: minmax(0, 1fr) minmax(120px, 0.5fr) minmax(0, 1fr);
  }
}
</style>
