<script setup lang="ts">
import type {
  ServiceAccount,
  ServiceAccountPayload,
  TokenPayload,
  TokenResponse,
} from '~/types/service-accounts'
import type {
  FirewallMoveDirection,
  FirewallRule,
  FirewallRulePayload,
  FirewallSimulationResponse,
} from '~/types/firewall'
import AdminServiceAccountDialog from '~/components/AdminServiceAccounts/AdminServiceAccountDialog.vue'
import AdminServiceAccountFirewallDialog from '~/components/AdminServiceAccounts/AdminServiceAccountFirewallDialog.vue'
import AdminServiceAccountTokenCreatedDialog from '~/components/AdminServiceAccounts/AdminServiceAccountTokenCreatedDialog.vue'
import AdminServiceAccountTokensDialog from '~/components/AdminServiceAccounts/AdminServiceAccountTokensDialog.vue'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Service accounts',
  icon: 'mdi-robot-outline',
  drawerIndex: 1,
})

const adminServiceAccounts = useAdminServiceAccounts()
const accountDialogOpen = shallowRef(false)
const firewallDialogOpen = shallowRef(false)
const tokenDialogOpen = shallowRef(false)
const tokenCreateDialogOpen = shallowRef(false)
const createdTokenDialogOpen = shallowRef(false)
const deleteDialog = useTargetDialog<ServiceAccount>()
const statusDialog = useTargetDialog<ServiceAccount>()
const firewallAccount = shallowRef<ServiceAccount | null>(null)
const tokenAccount = shallowRef<ServiceAccount | null>(null)
const showRevokedTokens = shallowRef(false)
const statusConfirm = useToggleConfirmDialog(statusDialog.target, {
  disableIcon: 'mdi-account-cancel-outline',
  enableIcon: 'mdi-account-check-outline',
  entityLabel: 'service account',
  fallbackMessage: 'Change this service account status.',
  isActive: (account) => account.isActive,
  name: (account) => account.name,
})

const totalLabel = computed(() => {
  const total = adminServiceAccounts.accounts.value.length
  const active = adminServiceAccounts.activeAccountsCount.value
  return total === 1 ? `${active}/1 active` : `${active}/${total} active`
})

// openCreateDialog prepares a blank service account form.
function openCreateDialog() {
  adminServiceAccounts.selectedAccount.value = null
  accountDialogOpen.value = true
}

// openEditDialog loads a service account before showing the edit dialog.
async function openEditDialog(account: ServiceAccount) {
  adminServiceAccounts.selectedAccount.value = account
  accountDialogOpen.value = true
  void adminServiceAccounts.loadAccount(account.id).catch(() => {})
}

// saveAccount creates or updates the active service account form.
async function saveAccount(payload: ServiceAccountPayload) {
  if (adminServiceAccounts.selectedAccount.value) {
    await adminServiceAccounts.updateAccount(
      adminServiceAccounts.selectedAccount.value.id,
      payload,
    )
  } else {
    await adminServiceAccounts.createAccount(payload)
  }

  accountDialogOpen.value = false
}

// openTokenDialog loads tokens before opening the service account token dialog.
async function openTokenDialog(account: ServiceAccount) {
  tokenAccount.value = account
  adminServiceAccounts.selectedAccount.value = account
  tokenDialogOpen.value = true
  tokenCreateDialogOpen.value = false
  showRevokedTokens.value = false
  adminServiceAccounts.tokens.value = []
  adminServiceAccounts.setTokenPage(1)
  void adminServiceAccounts
    .loadTokens(account.id, showRevokedTokens.value)
    .catch(() => {})
}

// openFirewallDialog loads scoped firewall rules for a service account.
async function openFirewallDialog(account: ServiceAccount) {
  firewallAccount.value = account
  adminServiceAccounts.selectedAccount.value = account
  firewallDialogOpen.value = true
  adminServiceAccounts.firewallRules.value = []
  adminServiceAccounts.setFirewallPage(1)
  void adminServiceAccounts.loadFirewallRules(account.id).catch(() => {})
}

// refreshFirewallRules reloads scoped firewall rows for the selected account.
async function refreshFirewallRules() {
  if (!firewallAccount.value) {
    return
  }

  await adminServiceAccounts.loadFirewallRules(firewallAccount.value.id)
}

// toggleFirewallOverride updates the selected account override flag.
async function toggleFirewallOverride(enabled: boolean) {
  if (!firewallAccount.value) {
    return
  }

  const updated = await adminServiceAccounts.updateAccount(
    firewallAccount.value.id,
    {
      identifier: firewallAccount.value.identifier,
      name: firewallAccount.value.name,
      isActive: firewallAccount.value.isActive,
      firewallOverrideEnabled: enabled,
    },
  )
  firewallAccount.value = updated
}

// createFirewallRule creates a scoped firewall rule.
async function createFirewallRule(payload: FirewallRulePayload) {
  if (!firewallAccount.value) {
    return
  }

  await adminServiceAccounts.createFirewallRule(
    firewallAccount.value.id,
    payload,
  )
}

// updateFirewallRule updates a scoped firewall rule.
async function updateFirewallRule(
  rule: FirewallRule,
  payload: FirewallRulePayload,
) {
  if (!firewallAccount.value) {
    return
  }

  await adminServiceAccounts.updateFirewallRule(
    firewallAccount.value.id,
    rule.id,
    payload,
  )
}

// deleteFirewallRule deletes a scoped firewall rule.
async function deleteFirewallRule(rule: FirewallRule) {
  if (!firewallAccount.value) {
    return
  }

  await adminServiceAccounts.deleteFirewallRule(
    firewallAccount.value.id,
    rule.id,
  )
}

// moveFirewallRule changes scoped firewall rule priority.
async function moveFirewallRule(
  rule: FirewallRule,
  direction: FirewallMoveDirection,
) {
  if (!firewallAccount.value) {
    return
  }

  await adminServiceAccounts.moveFirewallRulePriority(
    firewallAccount.value.id,
    rule.id,
    direction,
  )
}

// toggleFirewallRule flips scoped firewall rule enabled state.
async function toggleFirewallRule(rule: FirewallRule) {
  await updateFirewallRule(rule, {
    address: rule.address,
    description: rule.description,
    priority: rule.priority,
    action: rule.action,
    enabled: !rule.enabled,
  })
}

// simulateFirewallIp evaluates an IP against scoped firewall rules.
async function simulateFirewallIp(
  clientIp: string,
): Promise<FirewallSimulationResponse> {
  if (!firewallAccount.value) {
    throw new Error('Service account is required.')
  }

  return await adminServiceAccounts.simulateFirewallIp(
    firewallAccount.value.id,
    clientIp,
  )
}

// updateFirewallPage changes scoped firewall pagination and reloads rules.
async function updateFirewallPage(value: number) {
  adminServiceAccounts.setFirewallPage(value)
  await refreshFirewallRules()
}

// updateFirewallPageSize changes scoped firewall page size and reloads rules.
async function updateFirewallPageSize(value: number) {
  adminServiceAccounts.setFirewallPageSize(value)
  await refreshFirewallRules()
}

// updateFirewallSort changes scoped firewall sorting and reloads rules.
async function updateFirewallSort(sortBy: string, sortDir: 'asc' | 'desc') {
  adminServiceAccounts.setFirewallSort(sortBy, sortDir)
  await refreshFirewallRules()
}

// createToken creates a service account token and opens the secret dialog.
async function createToken(payload: TokenPayload) {
  if (!tokenAccount.value) {
    return
  }

  await adminServiceAccounts.createToken(
    tokenAccount.value.id,
    payload,
    showRevokedTokens.value,
  )
  tokenCreateDialogOpen.value = false
  createdTokenDialogOpen.value = true
}

watch(tokenDialogOpen, (open) => {
  if (!open) {
    tokenCreateDialogOpen.value = false
  }
})

// refreshTokens reloads tokens for the selected service account.
async function refreshTokens() {
  if (!tokenAccount.value) {
    return
  }

  await adminServiceAccounts.loadTokens(
    tokenAccount.value.id,
    showRevokedTokens.value,
  )
}

// updateTokenPage changes token pagination and reloads tokens.
async function updateTokenPage(value: number) {
  adminServiceAccounts.setTokenPage(value)
  await refreshTokens()
}

// updateTokenPageSize changes token page size and reloads tokens.
async function updateTokenPageSize(value: number) {
  adminServiceAccounts.setTokenPageSize(value)
  await refreshTokens()
}

// updateTokenSort changes token sorting and reloads tokens.
async function updateTokenSort(sortBy: string, sortDir: 'asc' | 'desc') {
  adminServiceAccounts.setTokenSort(sortBy, sortDir)
  await refreshTokens()
}

// revokeToken revokes one service account token.
async function revokeToken(token: TokenResponse) {
  if (!tokenAccount.value) {
    return
  }

  await adminServiceAccounts.revokeToken(
    tokenAccount.value.id,
    token.id,
    showRevokedTokens.value,
  )
}

// updateShowRevokedTokens toggles revoked-token visibility and reloads.
async function updateShowRevokedTokens(showRevoked: boolean) {
  showRevokedTokens.value = showRevoked
  await refreshTokens()
}

// confirmDelete removes the selected service account.
async function confirmDelete() {
  if (!deleteDialog.target.value) {
    return
  }

  await adminServiceAccounts.deleteAccount(deleteDialog.target.value.id)
  deleteDialog.close()
}

// confirmToggleStatus toggles the selected service account active state.
async function confirmToggleStatus() {
  if (!statusDialog.target.value) {
    return
  }

  const account = statusDialog.target.value
  await adminServiceAccounts.updateAccount(account.id, {
    identifier: account.identifier,
    name: account.name,
    isActive: !account.isActive,
  })
  statusDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-robot-outline"
          kicker="Automation access"
          title="Service accounts"
          copy="Manage non-human accounts and generate short-lived virtual keys for integrations."
          stat-label="Active accounts"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="adminServiceAccounts.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminServiceAccounts.listError.value }}
        </v-alert>

        <AdminServiceAccountsTable
          :items="adminServiceAccounts.accounts.value"
          :loading="adminServiceAccounts.loading.value"
          :page="adminServiceAccounts.page.value"
          :page-size="adminServiceAccounts.pageSize.value"
          :sort-by="adminServiceAccounts.sortBy.value"
          :sort-dir="adminServiceAccounts.sortDir.value"
          :total="adminServiceAccounts.total.value"
          @create="openCreateDialog"
          @delete="deleteDialog.open"
          @edit="openEditDialog"
          @manage-firewall="openFirewallDialog"
          @manage-tokens="openTokenDialog"
          @refresh="adminServiceAccounts.reload"
          @toggle-status="statusDialog.open"
          @update:page="adminServiceAccounts.setPage"
          @update:page-size="adminServiceAccounts.setPageSize"
          @update:sort="adminServiceAccounts.setSort"
        />
      </v-col>
    </v-row>

    <AdminServiceAccountDialog
      v-model="accountDialogOpen"
      :account="adminServiceAccounts.selectedAccount.value"
      :loading="adminServiceAccounts.saving.value"
      @save="saveAccount"
    />

    <AdminServiceAccountFirewallDialog
      v-model="firewallDialogOpen"
      :account="firewallAccount"
      :create-rule="createFirewallRule"
      :delete-rule="deleteFirewallRule"
      :loading="adminServiceAccounts.firewallLoading.value"
      :move-rule="moveFirewallRule"
      :next-priority="adminServiceAccounts.nextFirewallPriority.value"
      :page="adminServiceAccounts.firewallPage.value"
      :page-size="adminServiceAccounts.firewallPageSize.value"
      :refresh="refreshFirewallRules"
      :rules="adminServiceAccounts.firewallRules.value"
      :saving="adminServiceAccounts.saving.value"
      :simulate="simulateFirewallIp"
      :sort-by="adminServiceAccounts.firewallSortBy.value"
      :sort-dir="adminServiceAccounts.firewallSortDir.value"
      :toggle-override="toggleFirewallOverride"
      :toggle-rule="toggleFirewallRule"
      :total="adminServiceAccounts.firewallTotal.value"
      :update-rule="updateFirewallRule"
      @update:page="updateFirewallPage"
      @update:page-size="updateFirewallPageSize"
      @update:sort="updateFirewallSort"
    />

    <AdminServiceAccountTokensDialog
      v-model="tokenDialogOpen"
      v-model:show-revoked="showRevokedTokens"
      :account="tokenAccount"
      :loading="adminServiceAccounts.tokenLoading.value"
      :page="adminServiceAccounts.tokenPage.value"
      :page-size="adminServiceAccounts.tokenPageSize.value"
      :saving="adminServiceAccounts.saving.value"
      :sort-by="adminServiceAccounts.tokenSortBy.value"
      :sort-dir="adminServiceAccounts.tokenSortDir.value"
      :tokens="adminServiceAccounts.tokens.value"
      :total="adminServiceAccounts.tokenTotal.value"
      @create="tokenCreateDialogOpen = true"
      @refresh="refreshTokens"
      @revoke="revokeToken"
      @update:page="updateTokenPage"
      @update:page-size="updateTokenPageSize"
      @update:show-revoked="updateShowRevokedTokens"
      @update:sort="updateTokenSort"
    />

    <AppTokenCreateDialog
      v-model="tokenCreateDialogOpen"
      :default-lifetime="365"
      :loading="adminServiceAccounts.saving.value"
      name-placeholder="ci_token"
      subtitle="Generate a new virtual key for this service account."
      @create="createToken"
    />

    <AdminServiceAccountTokenCreatedDialog
      v-model="createdTokenDialogOpen"
      :created-token="adminServiceAccounts.createdToken.value"
    />

    <AppConfirmDialog
      v-model="deleteDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Delete account"
      icon="mdi-delete-outline"
      :loading="adminServiceAccounts.saving.value"
      :message="`Delete ${deleteDialog.target.value?.name ?? 'this service account'} and all of its virtual keys?`"
      title="Delete service account"
      @cancel="deleteDialog.close"
      @confirm="confirmDelete"
    />

    <AppConfirmDialog
      v-model="statusDialog.isOpen.value"
      :confirm-color="statusConfirm.confirmColor.value"
      :confirm-label="statusConfirm.actionLabel.value"
      :icon="statusConfirm.icon.value"
      :loading="adminServiceAccounts.saving.value"
      :message="statusConfirm.message.value"
      :title="statusConfirm.title.value"
      @cancel="statusDialog.close"
      @confirm="confirmToggleStatus"
    />
  </v-container>
</template>
