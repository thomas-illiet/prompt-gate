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
  <AppDialogCard v-model="isOpen" icon="mdi-account-multiple-plus-outline" :loading="props.loading" max-width="760" subtitle="Add users or service accounts to this access group." :title="title">
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
            height="48"
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
      <template #actions>
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
        <AppDialogCloseButton :disabled="props.loading" label="Close" @click="isOpen = false" />
      </template>
  </AppDialogCard>
</template>

<style scoped>
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
