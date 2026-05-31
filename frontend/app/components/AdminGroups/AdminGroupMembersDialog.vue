<script setup lang="ts">
import type { AccessGroup, GroupMemberSummary } from '~/types/groups'
import { appRoleColor, appRoleLabel } from '~/utils/auth'

const props = defineProps<{
  group: AccessGroup | null
  loading: boolean
  memberOptions: GroupMemberSummary[]
  optionsLoading: boolean
}>()

const emit = defineEmits<{
  add: [userId: string]
  refresh: []
  remove: [userId: string]
}>()

const isOpen = defineModel<boolean>({ default: false })
const selectedMemberId = shallowRef<string | null>(null)

const currentMemberIds = computed(
  () => new Set(props.group?.members.map((member) => member.id) ?? []),
)
const availableMembers = computed(() =>
  props.memberOptions
    .filter((member) => !currentMemberIds.value.has(member.id))
    .map((member) => ({
      title: displayMember(member),
      value: member.id,
      props: {
        subtitle:
          member.type === 'service'
            ? `service:${member.preferredUsername}`
            : member.email || member.preferredUsername,
      },
    })),
)
const members = computed(() => props.group?.members ?? [])
const title = computed(() =>
  props.group ? `Manage ${props.group.name} members` : 'Manage members',
)

watch(isOpen, (open) => {
  if (open) {
    selectedMemberId.value = null
  }
})

function displayMember(member: GroupMemberSummary) {
  return member.name || member.preferredUsername || member.email || member.id
}

function addMember() {
  if (!selectedMemberId.value) {
    return
  }
  emit('add', selectedMemberId.value)
  selectedMemberId.value = null
}
</script>

<template>
  <v-dialog v-model="isOpen" max-width="760" :persistent="props.loading">
    <v-card rounded="xl" class="admin-group-members-dialog">
      <v-card-item class="px-6 pt-6 pb-2">
        <template #prepend>
          <v-avatar color="primary" variant="tonal" size="44">
            <v-icon icon="mdi-account-multiple-plus-outline" />
          </v-avatar>
        </template>
        <v-card-title class="text-h6">{{ title }}</v-card-title>
        <v-card-subtitle>
          Add users or service accounts to this access group.
        </v-card-subtitle>
      </v-card-item>

      <v-card-text class="px-6 pb-2">
        <div class="admin-group-members-dialog__add">
          <v-select
            v-model="selectedMemberId"
            :items="availableMembers"
            label="Add member"
            variant="outlined"
            density="comfortable"
            :loading="props.optionsLoading"
            clearable
          />
          <v-btn
            color="primary"
            rounded="lg"
            prepend-icon="mdi-plus"
            :disabled="!selectedMemberId"
            :loading="props.loading"
            @click="addMember"
          >
            Add
          </v-btn>
        </div>

        <v-list
          v-if="members.length"
          bg-color="transparent"
          lines="two"
          class="admin-group-members-dialog__list"
        >
          <v-list-item
            v-for="member in members"
            :key="member.id"
            :title="displayMember(member)"
            :subtitle="
              member.type === 'service'
                ? `service:${member.preferredUsername}`
                : member.email || member.preferredUsername
            "
          >
            <template #prepend>
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon
                  :icon="
                    member.type === 'service'
                      ? 'mdi-robot-outline'
                      : 'mdi-account-outline'
                  "
                />
              </v-avatar>
            </template>

            <template #append>
              <v-chip
                size="small"
                label
                variant="tonal"
                :color="appRoleColor(member.role)"
                class="mr-2"
              >
                {{ appRoleLabel(member.role) }}
              </v-chip>
              <v-btn
                icon="mdi-close"
                variant="text"
                size="small"
                color="error"
                :loading="props.loading"
                :aria-label="`Remove ${displayMember(member)}`"
                @click="emit('remove', member.id)"
              />
            </template>
          </v-list-item>
        </v-list>

        <AppEmptyState
          v-else
          compact
          icon="mdi-account-multiple-remove-outline"
          title="No members"
          text="Add a user or service account before this group grants access."
        />
      </v-card-text>

      <v-card-actions class="px-6 pb-6">
        <v-spacer />
        <v-btn
          color="primary"
          variant="tonal"
          rounded="lg"
          prepend-icon="mdi-refresh"
          :loading="props.loading"
          @click="emit('refresh')"
        >
          Refresh
        </v-btn>
        <AppDialogCloseButton label="Close" @click="isOpen = false" />
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.admin-group-members-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
}

.admin-group-members-dialog__add {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: start;
}

.admin-group-members-dialog__list {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  border-radius: 8px;
}

@media (max-width: 720px) {
  .admin-group-members-dialog__add {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
