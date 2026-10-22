<script setup lang="ts">
import AdminAccountUsageDialog from '~/components/AdminAccounts/AdminAccountUsageDialog.vue'
import type { ServiceAccount } from '~/types/service-accounts'
import { adminServiceAccountPath } from '~/utils/api-paths'

const props = defineProps<{
  account: ServiceAccount
}>()

const isOpen = defineModel<boolean>({ default: false })
const endpoint = computed(() =>
  adminServiceAccountPath(props.account.id, 'statistics'),
)
const title = computed(() => `${props.account.name} usage statistics`)
</script>

<template>
  <AdminAccountUsageDialog
    v-model="isOpen"
    cost-caption="Service account usage estimate"
    empty-usage-text="This service account has no usage in the selected window."
    :endpoint="endpoint"
    subtitle="Usage attributed to this service account across the selected period."
    :title="title"
  />
</template>
