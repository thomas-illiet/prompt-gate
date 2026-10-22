<script setup lang="ts">
import AdminAccountUsageDialog from '~/components/AdminAccounts/AdminAccountUsageDialog.vue'
import type { AdminUser } from '~/types/users'
import { adminUserPath } from '~/utils/api-paths'

const props = defineProps<{
  user: AdminUser
}>()

const isOpen = defineModel<boolean>({ default: false })
const endpoint = computed(() => adminUserPath(props.user.id, 'statistics'))
const title = computed(() => `${displayUser(props.user)} usage statistics`)

// displayUser returns the most readable user identifier for the dialog title.
function displayUser(user: AdminUser) {
  return user.name || user.preferredUsername || user.email
}
</script>

<template>
  <AdminAccountUsageDialog
    v-model="isOpen"
    cost-caption="User usage estimate"
    empty-usage-text="This user has no usage in the selected window."
    :endpoint="endpoint"
    subtitle="Usage attributed to this user across the selected period."
    :title="title"
  />
</template>
