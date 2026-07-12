<script setup lang="ts">
import type { UserToken } from '~/types/user-service'
import type { AdminUser } from '~/types/users'
import type {
  FirewallMoveDirection,
  FirewallRule,
  FirewallRulePayload,
  FirewallSimulationResponse,
} from '~/types/firewall'
import AdminUserDeleteDialog from '~/components/AdminUsers/AdminUserDeleteDialog.vue'
import AdminUserEditDialog from '~/components/AdminUsers/AdminUserEditDialog.vue'
import AdminServiceAccountFirewallDialog from '~/components/AdminServiceAccounts/AdminServiceAccountFirewallDialog.vue'
import AdminUserGroupsDialog from '~/components/AdminUsers/AdminUserGroupsDialog.vue'
import AdminUserTokensDialog from '~/components/AdminUsers/AdminUserTokensDialog.vue'
import AdminAccountNoteDialog from '~/components/AdminAccounts/AdminAccountNoteDialog.vue'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'User management',
  icon: 'mdi-account-cog-outline',
  drawerIndex: 1,
  drawerSection: 'Identities',
})

const adminUsers = useAdminUsers()
const adminSubscriptions = useAdminSubscriptions()
const editDialogOpen = shallowRef(false)
const deleteDialogOpen = shallowRef(false)
const firewallDialogOpen = shallowRef(false)
const tokenDialogOpen = shallowRef(false)
const groupsDialogOpen = shallowRef(false)
const userToDelete = shallowRef<AdminUser | null>(null)
const firewallUser = shallowRef<AdminUser | null>(null)
const tokenUser = shallowRef<AdminUser | null>(null)
const groupsUser = shallowRef<AdminUser | null>(null)
const noteDialog = useTargetDialog<AdminUser>()
const statusDialog = useTargetDialog<AdminUser>()
const tokenRevokeDialog = useTargetDialog<UserToken>()
const statusConfirm = useToggleConfirmDialog(statusDialog.target, {
  disableIcon: 'mdi-account-cancel-outline',
  enableIcon: 'mdi-account-check-outline',
  entityLabel: 'user',
  fallbackMessage: 'Change this user status.',
  isActive: (user) => user.isActive,
  name: (user) => displayUser(user),
})

const totalLabel = computed(() =>
  adminUsers.total.value === 1
    ? '1 account'
    : `${adminUsers.total.value} accounts`,
)

const revokeTokenMessage = computed(() => {
  const tokenName = tokenRevokeDialog.target.value?.name ?? 'this virtual key'
  const userName = tokenUser.value ? displayUser(tokenUser.value) : 'this user'

  return `Revoke ${tokenName} for ${userName}? Existing clients using this virtual key will stop working.`
})

// displayUser returns the best available user identifier for dialogs.
function displayUser(user: AdminUser) {
  return user.name || user.preferredUsername || user.email
}

// openEditDialog loads a user before showing the edit dialog.
async function openEditDialog(user: AdminUser) {
  await Promise.all([
    adminUsers.loadUser(user.id),
    adminSubscriptions.loadAllPlans(),
  ])
  editDialogOpen.value = true
}

// saveUserAccess persists role, status, and expiration changes.
async function saveUserAccess(payload: {
  role: AdminUser['role']
  isActive: boolean
  expiresAt: string | null
  subscriptionPlanId: string | null
}) {
  if (!adminUsers.selectedUser.value) {
    return
  }

  await adminUsers.updateUserAccess(adminUsers.selectedUser.value.id, payload)
  editDialogOpen.value = false
}

// openDeleteDialog stores the user targeted for deletion.
function openDeleteDialog(user: AdminUser) {
  userToDelete.value = user
  deleteDialogOpen.value = true
}

// closeDeleteDialog clears deletion state.
function closeDeleteDialog() {
  deleteDialogOpen.value = false
  userToDelete.value = null
}

// openTokenDialog loads user tokens before showing token management.
function openTokenDialog(user: AdminUser) {
  tokenUser.value = user
  tokenDialogOpen.value = true
  adminUsers.tokens.value = []
  adminUsers.setTokenPage(1)
  void adminUsers.loadTokens(user.id).catch(() => {})
}

// openFirewallDialog loads scoped firewall rules for a user.
function openFirewallDialog(user: AdminUser) {
  firewallUser.value = user
  adminUsers.selectedUser.value = user
  firewallDialogOpen.value = true
  adminUsers.firewallRules.value = []
  adminUsers.setFirewallPage(1)
  void adminUsers.loadFirewallRules(user.id).catch(() => {})
}

// refreshFirewallRules reloads scoped firewall rows for the selected user.
async function refreshFirewallRules() {
  if (!firewallUser.value) {
    return
  }

  await adminUsers.loadFirewallRules(firewallUser.value.id)
}

// toggleFirewallOverride updates the selected user's firewall override flag.
async function toggleFirewallOverride(enabled: boolean) {
  if (!firewallUser.value) {
    return
  }

  const updated = await adminUsers.updateUser(firewallUser.value.id, {
    role: firewallUser.value.role,
    isActive: firewallUser.value.isActive,
    firewallOverrideEnabled: enabled,
    expiresAt: firewallUser.value.expiresAt,
  })
  firewallUser.value = updated
}

// createFirewallRule creates a scoped firewall rule.
async function createFirewallRule(payload: FirewallRulePayload) {
  if (!firewallUser.value) {
    return
  }

  await adminUsers.createFirewallRule(firewallUser.value.id, payload)
}

// updateFirewallRule updates a scoped firewall rule.
async function updateFirewallRule(
  rule: FirewallRule,
  payload: FirewallRulePayload,
) {
  if (!firewallUser.value) {
    return
  }

  await adminUsers.updateFirewallRule(firewallUser.value.id, rule.id, payload)
}

// deleteFirewallRule deletes a scoped firewall rule.
async function deleteFirewallRule(rule: FirewallRule) {
  if (!firewallUser.value) {
    return
  }

  await adminUsers.deleteFirewallRule(firewallUser.value.id, rule.id)
}

// moveFirewallRule changes scoped firewall rule priority.
async function moveFirewallRule(
  rule: FirewallRule,
  direction: FirewallMoveDirection,
) {
  if (!firewallUser.value) {
    return
  }

  await adminUsers.moveFirewallRulePriority(
    firewallUser.value.id,
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
  if (!firewallUser.value) {
    throw new Error('User is required.')
  }

  return await adminUsers.simulateFirewallIp(firewallUser.value.id, clientIp)
}

// updateFirewallPage changes scoped firewall pagination and reloads rules.
async function updateFirewallPage(value: number) {
  adminUsers.setFirewallPage(value)
  await refreshFirewallRules()
}

// updateFirewallPageSize changes scoped firewall page size and reloads rules.
async function updateFirewallPageSize(value: number) {
  adminUsers.setFirewallPageSize(value)
  await refreshFirewallRules()
}

// updateFirewallSort changes scoped firewall sorting and reloads rules.
async function updateFirewallSort(sortBy: string, sortDir: 'asc' | 'desc') {
  adminUsers.setFirewallSort(sortBy, sortDir)
  await refreshFirewallRules()
}

// openGroupsDialog loads group memberships before showing group management.
async function openGroupsDialog(user: AdminUser) {
  groupsUser.value = user
  await Promise.all([
    adminUsers.loadGroups(),
    adminUsers.loadUserGroups(user.id),
  ])
  groupsDialogOpen.value = true
}

// refreshTokens reloads token rows for the selected user.
async function refreshTokens() {
  if (!tokenUser.value) {
    return
  }

  await adminUsers.loadTokens(tokenUser.value.id)
}

// updateTokenPage changes token pagination and reloads rows.
async function updateTokenPage(value: number) {
  adminUsers.setTokenPage(value)
  await refreshTokens()
}

// updateTokenPageSize changes token page size and reloads rows.
async function updateTokenPageSize(value: number) {
  adminUsers.setTokenPageSize(value)
  await refreshTokens()
}

// updateTokenSort changes token sorting and reloads rows.
async function updateTokenSort(sortBy: string, sortDir: 'asc' | 'desc') {
  adminUsers.setTokenSort(sortBy, sortDir)
  await refreshTokens()
}

// confirmDelete deletes the selected user.
async function confirmDelete() {
  if (!userToDelete.value) {
    return
  }

  await adminUsers.deleteUser(userToDelete.value.id)
  closeDeleteDialog()
}

// confirmToggleStatus toggles the selected user's active state.
async function confirmToggleStatus() {
  if (!statusDialog.target.value) {
    return
  }

  const user = statusDialog.target.value
  await adminUsers.updateUser(user.id, {
    role: user.role,
    isActive: !user.isActive,
    expiresAt: user.expiresAt,
  })
  statusDialog.close()
}

// confirmRevokeToken revokes the selected user token.
async function confirmRevokeToken() {
  if (!tokenUser.value || !tokenRevokeDialog.target.value) {
    return
  }

  await adminUsers.revokeUserToken(
    tokenUser.value.id,
    tokenRevokeDialog.target.value.id,
  )
  tokenRevokeDialog.close()
}

// saveUserGroups replaces group memberships for the selected user.
async function saveUserGroups(groupIds: string[]) {
  if (!groupsUser.value) {
    return
  }

  await adminUsers.replaceUserGroups(groupsUser.value.id, groupIds)
  groupsDialogOpen.value = false
}

// saveUserNote persists the dedicated admin note for the selected user.
async function saveUserNote(note: string) {
  if (!noteDialog.target.value) {
    return
  }

  await adminUsers.updateUserNote(noteDialog.target.value.id, note)
  noteDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-account-cog-outline"
          kicker="Security control plane"
          title="User management"
          copy="Manage application roles, disable access, and monitor who has connected recently."
          stat-label="Total users"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <AdminUsersFilters
          :role="adminUsers.role.value"
          :search="adminUsers.search.value"
          :status="adminUsers.status.value"
          @update:role="adminUsers.setRole"
          @update:search="adminUsers.setSearch"
          @update:status="adminUsers.setStatus"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="adminUsers.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminUsers.listError.value }}
        </v-alert>

        <AdminUsersTable
          :items="adminUsers.users.value"
          :loading="adminUsers.loading.value"
          :page="adminUsers.page.value"
          :page-size="adminUsers.pageSize.value"
          :sort-by="adminUsers.sortBy.value"
          :sort-dir="adminUsers.sortDir.value"
          :total="adminUsers.total.value"
          @delete="openDeleteDialog"
          @edit="openEditDialog"
          @manage-firewall="openFirewallDialog"
          @manage-groups="openGroupsDialog"
          @manage-tokens="openTokenDialog"
          @notes="noteDialog.open"
          @refresh="adminUsers.reload"
          @toggle-status="statusDialog.open"
          @update:page="adminUsers.setPage"
          @update:page-size="adminUsers.setPageSize"
          @update:sort="adminUsers.setSort"
        />
      </v-col>
    </v-row>

    <AdminUserEditDialog
      v-model="editDialogOpen"
      :loading="adminUsers.saving.value"
      :subscription-plans="adminSubscriptions.plans.value"
      :user="adminUsers.selectedUser.value"
      @save="saveUserAccess"
    />
    <AdminUserDeleteDialog
      v-model="deleteDialogOpen"
      :loading="adminUsers.saving.value"
      :user="userToDelete"
      @cancel="closeDeleteDialog"
      @confirm="confirmDelete"
    />
    <AdminServiceAccountFirewallDialog
      v-model="firewallDialogOpen"
      :account="firewallUser"
      :create-rule="createFirewallRule"
      :delete-rule="deleteFirewallRule"
      :loading="adminUsers.firewallLoading.value"
      :move-rule="moveFirewallRule"
      :next-priority="adminUsers.nextFirewallPriority.value"
      :page="adminUsers.firewallPage.value"
      :page-size="adminUsers.firewallPageSize.value"
      :refresh="refreshFirewallRules"
      :rules="adminUsers.firewallRules.value"
      :saving="adminUsers.saving.value"
      :simulate="simulateFirewallIp"
      :sort-by="adminUsers.firewallSortBy.value"
      :sort-dir="adminUsers.firewallSortDir.value"
      :toggle-override="toggleFirewallOverride"
      :toggle-rule="toggleFirewallRule"
      :total="adminUsers.firewallTotal.value"
      :update-rule="updateFirewallRule"
      @update:page="updateFirewallPage"
      @update:page-size="updateFirewallPageSize"
      @update:sort="updateFirewallSort"
    />
    <AdminUserTokensDialog
      v-model="tokenDialogOpen"
      :loading="adminUsers.tokenLoading.value"
      :page="adminUsers.tokenPage.value"
      :page-size="adminUsers.tokenPageSize.value"
      :saving="adminUsers.saving.value"
      :sort-by="adminUsers.tokenSortBy.value"
      :sort-dir="adminUsers.tokenSortDir.value"
      :tokens="adminUsers.tokens.value"
      :total="adminUsers.tokenTotal.value"
      :user="tokenUser"
      @refresh="refreshTokens"
      @revoke="tokenRevokeDialog.open"
      @update:page="updateTokenPage"
      @update:page-size="updateTokenPageSize"
      @update:sort="updateTokenSort"
    />
    <AdminUserGroupsDialog
      v-model="groupsDialogOpen"
      :groups="adminUsers.groupOptions.value"
      :loading="adminUsers.groupLoading.value"
      :saving="adminUsers.saving.value"
      :selected-groups="adminUsers.userGroups.value"
      :user="groupsUser"
      @save="saveUserGroups"
    />
    <AdminAccountNoteDialog
      v-model="noteDialog.isOpen.value"
      :account="noteDialog.target.value"
      :loading="adminUsers.saving.value"
      @save="saveUserNote"
    />
    <AppConfirmDialog
      v-model="statusDialog.isOpen.value"
      :confirm-color="statusConfirm.confirmColor.value"
      :confirm-label="statusConfirm.actionLabel.value"
      :icon="statusConfirm.icon.value"
      :loading="adminUsers.saving.value"
      :message="statusConfirm.message.value"
      :title="statusConfirm.title.value"
      @cancel="statusDialog.close"
      @confirm="confirmToggleStatus"
    />
    <AppConfirmDialog
      v-model="tokenRevokeDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Revoke key"
      icon="mdi-key-remove"
      :loading="adminUsers.saving.value"
      :message="revokeTokenMessage"
      title="Revoke user virtual key"
      @cancel="tokenRevokeDialog.close"
      @confirm="confirmRevokeToken"
    />
  </v-container>
</template>
