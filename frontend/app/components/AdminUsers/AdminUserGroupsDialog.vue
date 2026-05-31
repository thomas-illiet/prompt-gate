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
  <v-dialog v-model="isOpen" max-width="640" :persistent="props.saving">
    <v-card rounded="xl" class="admin-user-groups-dialog">
      <v-card-item class="px-6 pt-6 pb-2">
        <template #prepend>
          <v-avatar color="primary" variant="tonal" size="44">
            <v-icon icon="mdi-account-multiple-check-outline" />
          </v-avatar>
        </template>

        <v-card-title class="text-h6">{{ title }}</v-card-title>
        <v-card-subtitle>
          Group membership controls proxy provider and model access.
        </v-card-subtitle>
      </v-card-item>

      <v-card-text class="px-6 pb-2">
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
      </v-card-text>

      <v-card-actions class="px-6 pb-6">
        <v-spacer />
        <AppDialogCloseButton label="Cancel" @click="isOpen = false" />
        <AppDialogActionButton
          color="primary"
          label="Save groups"
          :loading="props.saving"
          @click="save"
        />
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.admin-user-groups-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
}
</style>
