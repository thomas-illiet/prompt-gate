<script setup lang="ts">
import type { FirewallMoveDirection, FirewallRule } from '~/types/firewall'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Firewall',
  icon: 'mdi-shield-lock-outline',
  drawerIndex: 2,
})

const firewall = useAdminFirewall()
const ruleDialogOpen = shallowRef(false)
const simulatorDialogOpen = shallowRef(false)
const deleteDialog = useTargetDialog<FirewallRule>()
const toggleDialog = useTargetDialog<FirewallRule>()
const toggleConfirm = useToggleConfirmDialog(toggleDialog.target, {
  disableIcon: 'mdi-shield-off-outline',
  enableIcon: 'mdi-shield-check-outline',
  entityLabel: 'firewall rule',
  fallbackMessage: 'Change this firewall rule status.',
  isActive: (rule) => rule.enabled,
  name: (rule) => rule.address,
})

const totalLabel = computed(() => {
  const total = firewall.rules.value.length
  const enabled = firewall.enabledRulesCount.value
  return total === 1 ? `${enabled}/1 enabled` : `${enabled}/${total} enabled`
})

// openCreateDialog prepares a new firewall rule with the next priority.
function openCreateDialog() {
  firewall.selectedRule.value = null
  ruleDialogOpen.value = true
}

// openEditDialog loads a firewall rule before showing the edit dialog.
async function openEditDialog(rule: FirewallRule) {
  await firewall.loadRule(rule.id)
  ruleDialogOpen.value = true
}

// saveRule creates or updates the active firewall rule form.
async function saveRule(payload: {
  address: string
  description: string
  priority: number
  action: FirewallRule['action']
  enabled: boolean
}) {
  if (firewall.selectedRule.value) {
    await firewall.updateRule(firewall.selectedRule.value.id, payload)
  } else {
    await firewall.createRule(payload)
  }

  ruleDialogOpen.value = false
}

// moveRule requests a priority move for the selected firewall rule.
async function moveRule(rule: FirewallRule, direction: FirewallMoveDirection) {
  await firewall.moveRulePriority(rule.id, direction)
}

// confirmToggleRule toggles the selected firewall rule enabled state.
async function confirmToggleRule() {
  if (!toggleDialog.target.value) {
    return
  }

  const rule = toggleDialog.target.value
  await firewall.updateRule(rule.id, {
    address: rule.address,
    description: rule.description,
    priority: rule.priority,
    action: rule.action,
    enabled: !rule.enabled,
  })
  toggleDialog.close()
}

// openSimulatorDialog shows the firewall simulation dialog.
function openSimulatorDialog() {
  simulatorDialogOpen.value = true
}

// confirmDelete removes the selected firewall rule.
async function confirmDelete() {
  if (!deleteDialog.target.value) {
    return
  }

  await firewall.deleteRule(deleteDialog.target.value.id)
  deleteDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-shield-lock-outline"
          kicker="Security control plane"
          title="Firewall"
          copy="Manage IPv4 and CIDR rules evaluated by priority before proxy traffic reaches the application."
          stat-label="Enabled rules"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="firewall.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ firewall.listError.value }}
        </v-alert>

        <AdminFirewallTable
          :items="firewall.rules.value"
          :loading="firewall.loading.value"
          :page="firewall.page.value"
          :page-size="firewall.pageSize.value"
          :sort-by="firewall.sortBy.value"
          :sort-dir="firewall.sortDir.value"
          :total="firewall.total.value"
          @create="openCreateDialog"
          @delete="deleteDialog.open"
          @edit="openEditDialog"
          @move="moveRule"
          @refresh="firewall.reload"
          @simulate="openSimulatorDialog"
          @toggle="toggleDialog.open"
          @update:page="firewall.setPage"
          @update:page-size="firewall.setPageSize"
          @update:sort="firewall.setSort"
        />
      </v-col>
    </v-row>

    <AdminFirewallRuleDialog
      v-model="ruleDialogOpen"
      :default-priority="firewall.nextPriority.value"
      :loading="firewall.saving.value"
      :rule="firewall.selectedRule.value"
      @save="saveRule"
    />

    <AppDialogCard
      v-model="simulatorDialogOpen"
      icon="mdi-radar"
      max-width="900"
      title="Firewall simulator"
      subtitle="Check how backend firewall rules evaluate a client IPv4 address."
    >
      <AdminFirewallSimulator :simulate="firewall.simulateIp" />

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
      :loading="firewall.saving.value"
      :message="
        deleteDialog.target.value
          ? `Delete firewall rule ${deleteDialog.target.value.address}`
          : 'Delete this firewall rule.'
      "
      title="Delete firewall rule"
      @cancel="deleteDialog.close"
      @confirm="confirmDelete"
    />

    <AppConfirmDialog
      v-model="toggleDialog.isOpen.value"
      :confirm-color="toggleConfirm.confirmColor.value"
      :confirm-label="toggleConfirm.actionLabel.value"
      :icon="toggleConfirm.icon.value"
      :loading="firewall.saving.value"
      :message="toggleConfirm.message.value"
      :title="toggleConfirm.title.value"
      @cancel="toggleDialog.close"
      @confirm="confirmToggleRule"
    />
  </v-container>
</template>
