<script setup lang="ts">
import type {
  FirewallMoveDirection,
  FirewallRule,
  FirewallRulePayload,
  FirewallSimulationResponse,
} from '~/types/firewall'
import AdminFirewallRuleDialog from '~/components/AdminFirewall/AdminFirewallRuleDialog.vue'
import AdminFirewallSimulator from '~/components/AdminFirewall/AdminFirewallSimulator.vue'
import AdminServiceAccountFirewallRulesTable from '~/components/AdminServiceAccounts/AdminServiceAccountFirewallRulesTable.vue'

interface FirewallAccount {
  email?: string
  firewallOverrideEnabled: boolean
  identifier?: string
  name?: string
  preferredUsername?: string
}

const props = defineProps<{
  account: FirewallAccount | null
  createRule: (payload: FirewallRulePayload) => Promise<unknown>
  deleteRule: (rule: FirewallRule) => Promise<unknown>
  loading: boolean
  moveRule: (
    rule: FirewallRule,
    direction: FirewallMoveDirection,
  ) => Promise<unknown>
  nextPriority: number
  page: number
  pageSize: number
  refresh: () => Promise<unknown>
  rules: FirewallRule[]
  saving: boolean
  simulate: (clientIp: string) => Promise<FirewallSimulationResponse>
  sortBy: string
  sortDir: 'asc' | 'desc'
  toggleOverride: (enabled: boolean) => Promise<unknown>
  toggleRule: (rule: FirewallRule) => Promise<unknown>
  total: number
  updateRule: (
    rule: FirewallRule,
    payload: FirewallRulePayload,
  ) => Promise<unknown>
}>()

const emit = defineEmits<{
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const isOpen = defineModel<boolean>({ default: false })
const ruleDialogOpen = shallowRef(false)
const simulatorDialogOpen = shallowRef(false)
const editingRule = shallowRef<FirewallRule | null>(null)
const deleteDialog = useTargetDialog<FirewallRule>()

const title = computed(() =>
  props.account
    ? `${displayAccount(props.account)} firewall`
    : 'Account firewall',
)
const overrideEnabled = computed(
  () => props.account?.firewallOverrideEnabled ?? false,
)
const accountLabel = computed(() =>
  props.account
    ? props.account.identifier ||
      props.account.preferredUsername ||
      props.account.email ||
      'account'
    : 'account',
)

// displayAccount returns the most readable account name for dialog titles.
function displayAccount(account: FirewallAccount) {
  return (
    account.name ||
    account.identifier ||
    account.preferredUsername ||
    account.email ||
    'Account'
  )
}

// openCreateRuleDialog prepares a blank scoped firewall rule form.
function openCreateRuleDialog() {
  editingRule.value = null
  ruleDialogOpen.value = true
}

// openEditRuleDialog opens the scoped firewall rule form for one rule.
function openEditRuleDialog(rule: FirewallRule) {
  editingRule.value = rule
  ruleDialogOpen.value = true
}

// saveRule persists a created or edited scoped firewall rule.
async function saveRule(payload: FirewallRulePayload) {
  try {
    if (editingRule.value) {
      await props.updateRule(editingRule.value, payload)
    } else {
      await props.createRule(payload)
    }

    ruleDialogOpen.value = false
  } catch {
    return
  }
}

// handleToggleOverride updates the service-account firewall override flag.
async function handleToggleOverride(value: boolean | null) {
  try {
    await props.toggleOverride(Boolean(value))
  } catch {
    return
  }
}

// handleMoveRule updates scoped firewall priority.
async function handleMoveRule(
  rule: FirewallRule,
  direction: FirewallMoveDirection,
) {
  try {
    await props.moveRule(rule, direction)
  } catch {
    return
  }
}

// handleToggleRule flips scoped firewall rule enabled state.
async function handleToggleRule(rule: FirewallRule) {
  try {
    await props.toggleRule(rule)
  } catch {
    return
  }
}

// confirmDelete removes the selected scoped firewall rule.
async function confirmDelete() {
  if (!deleteDialog.target.value) {
    return
  }

  try {
    await props.deleteRule(deleteDialog.target.value)
    deleteDialog.close()
  } catch {
    return
  }
}

// refreshRules reloads scoped firewall rules.
async function refreshRules() {
  try {
    await props.refresh()
  } catch {
    return
  }
}
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    icon="mdi-shield-account-outline"
    max-width="1080"
    :loading="props.saving"
    :subtitle="accountLabel"
    :title="title"
  >
    <div class="service-account-firewall-dialog">
      <div class="service-account-firewall-dialog__toolbar">
        <div class="service-account-firewall-dialog__status">
          <v-switch
            color="primary"
            density="compact"
            hide-details
            inset
            label="Firewall override"
            :disabled="!props.account || props.saving"
            :model-value="overrideEnabled"
            @update:model-value="handleToggleOverride"
          />

          <v-chip
            size="small"
            label
            variant="tonal"
            :color="overrideEnabled ? 'success' : 'default'"
          >
            {{ overrideEnabled ? 'Override on' : 'Global firewall' }}
          </v-chip>
        </div>

        <div class="service-account-firewall-dialog__actions">
          <v-btn
            color="primary"
            variant="tonal"
            rounded="xl"
            prepend-icon="mdi-radar"
            :disabled="!props.account"
            @click="simulatorDialogOpen = true"
          >
            Simulate IP
          </v-btn>

          <v-btn
            color="primary"
            rounded="xl"
            prepend-icon="mdi-plus"
            :disabled="!props.account"
            @click="openCreateRuleDialog"
          >
            Create rule
          </v-btn>

          <v-btn
            color="primary"
            variant="tonal"
            rounded="xl"
            prepend-icon="mdi-refresh"
            :loading="props.loading"
            :disabled="!props.account"
            @click="refreshRules"
          >
            Refresh
          </v-btn>
        </div>
      </div>

      <AdminServiceAccountFirewallRulesTable
        :items="props.rules"
        :loading="props.loading"
        :page="props.page"
        :page-size="props.pageSize"
        :sort-by="props.sortBy"
        :sort-dir="props.sortDir"
        :total="props.total"
        @delete="deleteDialog.open"
        @edit="openEditRuleDialog"
        @move="handleMoveRule"
        @toggle="handleToggleRule"
        @update:page="emit('update:page', $event)"
        @update:page-size="emit('update:page-size', $event)"
        @update:sort="
          (nextSortBy, nextSortDir) =>
            emit('update:sort', nextSortBy, nextSortDir)
        "
      />
    </div>

    <AdminFirewallRuleDialog
      v-model="ruleDialogOpen"
      :default-priority="props.nextPriority"
      :loading="props.saving"
      :rule="editingRule"
      @save="saveRule"
    />

    <AppDialogCard
      v-model="simulatorDialogOpen"
      icon="mdi-radar"
      max-width="900"
      title="Firewall simulator"
      :subtitle="accountLabel"
    >
      <AdminFirewallSimulator :simulate="props.simulate" />

      <template #actions>
        <v-spacer />
        <AppDialogCloseButton @click="simulatorDialogOpen = false" />
      </template>
    </AppDialogCard>

    <AppConfirmDialog
      v-model="deleteDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Delete rule"
      icon="mdi-delete-alert-outline"
      :loading="props.saving"
      :message="
        deleteDialog.target.value
          ? `Delete firewall rule ${deleteDialog.target.value.address}`
          : 'Delete this firewall rule.'
      "
      title="Delete firewall rule"
      @cancel="deleteDialog.close"
      @confirm="confirmDelete"
    />

    <template #actions>
      <v-spacer />
      <AppDialogCloseButton @click="isOpen = false" />
    </template>
  </AppDialogCard>
</template>

<style scoped>
.service-account-firewall-dialog {
  display: grid;
  gap: 16px;
}

.service-account-firewall-dialog__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.service-account-firewall-dialog__status,
.service-account-firewall-dialog__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

@media (max-width: 720px) {
  .service-account-firewall-dialog__toolbar {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
