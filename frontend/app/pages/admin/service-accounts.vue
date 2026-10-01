<script setup lang="ts">
import type {
  ServiceAccount,
  ServiceAccountFormPayload,
  TokenPayload,
  TokenResponse,
} from '~/types/service-accounts'
import AdminServiceAccountDialog from '~/components/AdminServiceAccounts/AdminServiceAccountDialog.vue'
import AdminServiceAccountTokenCreatedDialog from '~/components/AdminServiceAccounts/AdminServiceAccountTokenCreatedDialog.vue'
import AdminServiceAccountTokensDialog from '~/components/AdminServiceAccounts/AdminServiceAccountTokensDialog.vue'
import AdminAccountNoteDialog from '~/components/AdminAccounts/AdminAccountNoteDialog.vue'
import AdminAccountIPAddressesDialog from '~/components/AdminAccounts/AdminAccountIPAddressesDialog.vue'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Service accounts',
  icon: 'mdi-robot-outline',
  drawerIndex: 2,
  drawerSection: 'Identities',
})

const adminServiceAccounts = useAdminServiceAccounts()
const adminSubscriptions = useAdminSubscriptions()
const accountDialogOpen = shallowRef(false)
const tokenDialogOpen = shallowRef(false)
const tokenCreateDialogOpen = shallowRef(false)
const createdTokenDialogOpen = shallowRef(false)
const deleteDialog = useTargetDialog<ServiceAccount>()
const noteDialog = useTargetDialog<ServiceAccount>()
const statusDialog = useTargetDialog<ServiceAccount>()
const ipDialog = useTargetDialog<ServiceAccount>()
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
  void adminSubscriptions.loadAllPlans().catch(() => {})
  accountDialogOpen.value = true
}

// openEditDialog loads a service account before showing the edit dialog.
async function openEditDialog(account: ServiceAccount) {
  adminServiceAccounts.selectedAccount.value = account
  await adminSubscriptions.loadAllPlans()
  accountDialogOpen.value = true
  void adminServiceAccounts.loadAccount(account.id).catch(() => {})
}

// saveAccount creates or updates the active service account form.
async function saveAccount(payload: ServiceAccountFormPayload) {
  if (adminServiceAccounts.selectedAccount.value) {
    await adminServiceAccounts.updateAccountWithSubscription(
      adminServiceAccounts.selectedAccount.value.id,
      payload,
    )
  } else {
    await adminServiceAccounts.createAccountWithSubscription(payload)
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

// openIPDialog loads observed proxy addresses before showing the dialog.
function openIPDialog(account: ServiceAccount) {
  ipDialog.open(account)
  adminServiceAccounts.ipAddresses.value = []
  adminServiceAccounts.setIPPage(1)
  void adminServiceAccounts.loadIPAddresses(account.id).catch(() => {})
}

async function refreshIPAddresses() {
  if (ipDialog.target.value) await adminServiceAccounts.loadIPAddresses(ipDialog.target.value.id)
}

async function updateIPPage(value: number) {
  adminServiceAccounts.setIPPage(value)
  await refreshIPAddresses()
}

async function updateIPPageSize(value: number) {
  adminServiceAccounts.setIPPageSize(value)
  await refreshIPAddresses()
}

async function updateIPSort(sortBy: string, sortDir: 'asc' | 'desc') {
  adminServiceAccounts.setIPSort(sortBy, sortDir)
  await refreshIPAddresses()
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

// saveAccountNote persists the dedicated admin note for the selected account.
async function saveAccountNote(note: string) {
  if (!noteDialog.target.value) {
    return
  }

  await adminServiceAccounts.updateAccountNote(noteDialog.target.value.id, note)
  noteDialog.close()
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
          @manage-ips="openIPDialog"
          @manage-tokens="openTokenDialog"
          @notes="noteDialog.open"
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
      :subscription-plans="adminSubscriptions.plans.value"
      @save="saveAccount"
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

    <AdminAccountNoteDialog
      v-model="noteDialog.isOpen.value"
      :account="noteDialog.target.value"
      :loading="adminServiceAccounts.saving.value"
      @save="saveAccountNote"
    />
    <AdminAccountIPAddressesDialog
      v-model="ipDialog.isOpen.value"
      :account-name="ipDialog.target.value?.name ?? 'Service account'"
      :items="adminServiceAccounts.ipAddresses.value"
      :loading="adminServiceAccounts.ipLoading.value"
      :page="adminServiceAccounts.ipPage.value"
      :page-size="adminServiceAccounts.ipPageSize.value"
      :sort-by="adminServiceAccounts.ipSortBy.value"
      :sort-dir="adminServiceAccounts.ipSortDir.value"
      :total="adminServiceAccounts.ipTotal.value"
      @refresh="refreshIPAddresses"
      @update:page="updateIPPage"
      @update:page-size="updateIPPageSize"
      @update:sort="updateIPSort"
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
