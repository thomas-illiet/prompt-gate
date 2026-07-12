<script setup lang="ts">
import type { AccessGroup } from '~/types/groups'
import type { AdminUser } from '~/types/users'

const props = defineProps<{
  groups: AccessGroup[]
  loading: boolean
  saving: boolean
  selectedGroups: AccessGroup[]
  user: AdminUser | null
}>()

const emit = defineEmits<{
  save: [groupIds: string[]]
}>()

const isOpen = defineModel<boolean>({ default: false })
const selectedGroupIds = shallowRef<string[]>([])

const groupItems = computed(() =>
  props.groups.map((group) => ({
    title: group.displayName || group.name,
    value: group.id,
    props: {
      subtitle: group.name,
    },
  })),
)
const title = computed(() =>
  props.user
    ? `Manage ${displayUser(props.user)} groups`
    : 'Manage user groups',
)

watch(
  [isOpen, () => props.selectedGroups],
  ([open]) => {
    if (!open) {
      return
    }
    selectedGroupIds.value = props.selectedGroups.map((group) => group.id)
  },
  { immediate: true },
)

function displayUser(user: AdminUser) {
  return user.name || user.preferredUsername || user.email
}

function save() {
  emit('save', selectedGroupIds.value)
}
</script>

<template>
  <AppDialogCard v-model="isOpen" icon="mdi-account-multiple-check-outline" :loading="props.saving" max-width="640" subtitle="Group membership controls provider and model access." :title="title">
        <v-select
          v-model="selectedGroupIds"
          :items="groupItems"
          label="Groups"
          variant="outlined"
          density="comfortable"
          multiple
          chips
          closable-chips
          :loading="props.loading"
        />
      <template #actions>
        <AppDialogCloseButton :disabled="props.saving" label="Cancel" @click="isOpen = false" />
        <AppDialogActionButton
          color="primary"
          label="Save groups"
          :loading="props.saving"
          @click="save"
        />
      </template>
  </AppDialogCard>
</template>
