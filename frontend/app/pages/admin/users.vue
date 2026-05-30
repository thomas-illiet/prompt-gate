<script setup lang="ts">
import type { UserToken } from '~/types/user-service'
import type { AdminUser } from '~/types/users'
import AdminUserDeleteDialog from '~/components/AdminUsers/AdminUserDeleteDialog.vue'
import AdminUserEditDialog from '~/components/AdminUsers/AdminUserEditDialog.vue'
import AdminUserTokensDialog from '~/components/AdminUsers/AdminUserTokensDialog.vue'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'User management',
  icon: 'mdi-account-cog-outline',
})

const adminUsers = useAdminUsers()
const editDialogOpen = shallowRef(false)
const deleteDialogOpen = shallowRef(false)
const tokenDialogOpen = shallowRef(false)
const userToDelete = shallowRef<AdminUser | null>(null)
const tokenUser = shallowRef<AdminUser | null>(null)
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
  await adminUsers.loadUser(user.id)
  editDialogOpen.value = true
}

// saveUserAccess persists role, status, and expiration changes.
async function saveUserAccess(payload: {
  role: AdminUser['role']
  isActive: boolean
  expiresAt: string | null
}) {
  if (!adminUsers.selectedUser.value) {
    return
  }

  await adminUsers.updateUser(adminUsers.selectedUser.value.id, payload)
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
          @manage-tokens="openTokenDialog"
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
