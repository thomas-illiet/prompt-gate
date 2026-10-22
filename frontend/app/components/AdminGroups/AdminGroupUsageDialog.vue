<script setup lang="ts">
import AdminAccountUsageDialog from '~/components/AdminAccounts/AdminAccountUsageDialog.vue'
import type { AccessGroup } from '~/types/groups'
import { adminGroupPath } from '~/utils/api-paths'

const props = defineProps<{
  group: AccessGroup
}>()

const isOpen = defineModel<boolean>({ default: false })
const endpoint = computed(() => adminGroupPath(props.group.id, 'statistics'))
const title = computed(
  () => `${props.group.displayName || props.group.name} usage statistics`,
)
</script>

<template>
  <AdminAccountUsageDialog
    v-model="isOpen"
    cost-caption="Group usage estimate"
    empty-usage-text="This group has no usage in the selected window."
    :endpoint="endpoint"
    subtitle="Usage attributed to the current members of this group across the selected period."
    :title="title"
  />
</template>
