<script setup lang="ts">
import UserTokenCreatedDialog from '~/components/UserTokens/UserTokenCreatedDialog.vue'
import UserTokenFilters from '~/components/UserTokens/UserTokenFilters.vue'
import UserTokenTable from '~/components/UserTokens/UserTokenTable.vue'
import type { UserToken, UserTokenPayload } from '~/types/user-service'
import { USER_TOKEN_MAX_LIFETIME_DAYS } from '~/utils/user-tokens'

definePageMeta({
  icon: 'mdi-key-outline',
  path: '/tokens',
  title: 'Virtual keys',
  drawerIndex: 2,
  requiredRoles: ['user', 'admin'],
})

const userTokens = useUserTokens()
const createDialogOpen = shallowRef(false)
const createdTokenDialogOpen = shallowRef(false)
const revokeDialog = useTargetDialog<UserToken>()

const filteredTokenStatLabel = computed(() => {
  const label = userTokens.statusFilter.value
  return `${label.charAt(0).toUpperCase()}${label.slice(1)} keys`
})
const filteredTokenStatValue = computed(() =>
  userTokens.loading.value ? '…' : String(userTokens.total.value),
)

// createToken submits the token form and opens the one-time secret dialog.
async function createToken(payload: UserTokenPayload) {
  await userTokens.createToken(payload)
  createDialogOpen.value = false
  createdTokenDialogOpen.value = true
}

// openCreateDialog resets token creation state and shows the dialog.
function openCreateDialog() {
  createDialogOpen.value = true
}

// confirmRevoke revokes the selected token and closes the confirmation dialog.
async function confirmRevoke() {
  if (!revokeDialog.target.value) {
    return
  }

  await userTokens.revokeToken(revokeDialog.target.value.id)
  revokeDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHeader
          icon="mdi-key-outline"
          kicker="The keyring"
          title="Virtual keys"
          copy="Keep access on your terms. Create and revoke virtual keys for your own service access."
          visual="key"
          :stat-label="filteredTokenStatLabel"
          :stat-busy="userTokens.loading.value"
          :stat-value="filteredTokenStatValue"
        />
      </v-col>

      <v-col cols="12">
        <UserTokenFilters
          :search="userTokens.search.value"
          :status-filter="userTokens.statusFilter.value"
          @update:search="userTokens.setSearch"
          @update:status-filter="userTokens.setStatusFilter"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="userTokens.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          <div class="tokens-page__error">
            <span>{{ userTokens.listError.value }}</span>
            <v-btn
              data-test="tokens-retry"
              prepend-icon="mdi-refresh"
              size="small"
              variant="text"
              @click="userTokens.reload"
            >
              Retry
            </v-btn>
          </div>
        </v-alert>

        <UserTokenTable
          :items="userTokens.tokens.value"
          :loading="userTokens.loading.value"
          :page="userTokens.page.value"
          :page-size="userTokens.pageSize.value"
          :saving="userTokens.saving.value"
          :sort-by="userTokens.sortBy.value"
          :sort-dir="userTokens.sortDir.value"
          :total="userTokens.total.value"
          @create="openCreateDialog"
          @refresh="userTokens.reload"
          @revoke="revokeDialog.open"
          @update:page="userTokens.setPage"
          @update:page-size="userTokens.setPageSize"
          @update:sort="userTokens.setSort"
        />
      </v-col>
    </v-row>

    <AppTokenCreateDialog
      v-model="createDialogOpen"
      :loading="userTokens.saving.value"
      :max-lifetime="USER_TOKEN_MAX_LIFETIME_DAYS"
      @create="createToken"
    />

    <UserTokenCreatedDialog
      v-model="createdTokenDialogOpen"
      :created-token="userTokens.createdToken.value"
    />

    <AppConfirmDialog
      v-model="revokeDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Revoke key"
      icon="mdi-key-remove"
      :loading="userTokens.saving.value"
      :message="`Revoke ${revokeDialog.target.value?.name ?? 'this virtual key'}? Existing clients using it will lose access.`"
      title="Revoke virtual key"
      @cancel="revokeDialog.close"
      @confirm="confirmRevoke"
    />
  </v-container>
</template>

<style scoped>
.tokens-page__error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

@media (max-width: 600px) {
  .tokens-page__error {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
