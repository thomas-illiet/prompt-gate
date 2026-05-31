<script setup lang="ts">
import AdminGroupMembersList from '~/components/AdminGroups/AdminGroupMembersList.vue'
import type { AccessGroup, GroupMemberSummary } from '~/types/groups'

interface AvailableMemberItem {
  title: string
  value: string
  searchText: string
  props: {
    subtitle: string
  }
}

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
const selectedMemberSearch = shallowRef('')
const memberOptionFilterKeys = ['title', 'searchText']

const currentMemberIds = computed(
  () => new Set(props.group?.members.map((member) => member.id) ?? []),
)
const availableMembers = computed<AvailableMemberItem[]>(() =>
  props.memberOptions
    .filter((member) => !currentMemberIds.value.has(member.id))
    .map((member) => ({
      title: displayMember(member),
      value: member.id,
      searchText: memberSearchText(member),
      props: {
        subtitle: memberSubtitle(member),
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
    selectedMemberSearch.value = ''
  }
})

function displayMember(member: GroupMemberSummary) {
  return member.name || member.preferredUsername || member.email || member.id
}

function memberSubtitle(member: GroupMemberSummary) {
  return member.type === 'service'
    ? `service:${member.preferredUsername}`
    : member.email || member.preferredUsername
}

function memberSearchText(member: GroupMemberSummary) {
  return [
    displayMember(member),
    memberSubtitle(member),
    member.email,
    member.preferredUsername,
    member.role,
    member.type,
    member.id,
  ]
    .join(' ')
    .trim()
    .toLowerCase()
}

function addMember() {
  if (!selectedMemberId.value) {
    return
  }
  emit('add', selectedMemberId.value)
  selectedMemberId.value = null
  selectedMemberSearch.value = ''
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
          <v-autocomplete
            v-model="selectedMemberId"
            v-model:search="selectedMemberSearch"
            :items="availableMembers"
            :filter-keys="memberOptionFilterKeys"
            label="Add member"
            variant="outlined"
            density="comfortable"
            :loading="props.optionsLoading"
            auto-select-first
            clearable
            no-data-text="No available members match your search"
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

        <AdminGroupMembersList
          :active="isOpen"
          :loading="props.loading"
          :members="members"
          @remove="emit('remove', $event)"
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
  margin-bottom: 16px;
}

@media (max-width: 720px) {
  .admin-group-members-dialog__add {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
