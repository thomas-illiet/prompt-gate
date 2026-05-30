<script setup lang="ts">
import UserTokenCreatedDialog from '~/components/UserTokens/UserTokenCreatedDialog.vue'
import UserTokenFilters from '~/components/UserTokens/UserTokenFilters.vue'
import UserTokenTable from '~/components/UserTokens/UserTokenTable.vue'
import type { UserToken, UserTokenPayload } from '~/types/user-service'

definePageMeta({
  icon: 'mdi-key-outline',
  path: '/tokens',
  title: 'Virtual keys',
  drawerIndex: 2,
  requiredRoles: ['user', 'manager', 'admin'],
})

const userTokens = useUserTokens()
const createDialogOpen = shallowRef(false)
const createdTokenDialogOpen = shallowRef(false)
const revokeDialog = useTargetDialog<UserToken>()

const filteredTokenLabel = computed(
  () => `${userTokens.total.value} ${userTokens.statusFilter.value}`,
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
        <AppPageHero
          icon="mdi-key-outline"
          kicker="Personal access"
          title="Virtual keys"
          copy="Create and revoke virtual keys for your own service access."
          stat-label="Keys"
          :stat-value="filteredTokenLabel"
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
          {{ userTokens.listError.value }}
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
