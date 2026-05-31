<script setup lang="ts">
import type { GroupMemberSummary } from '~/types/groups'
import { appRoleColor, appRoleLabel } from '~/utils/auth'

const props = defineProps<{
  active: boolean
  loading: boolean
  members: GroupMemberSummary[]
}>()

const emit = defineEmits<{
  remove: [userId: string]
}>()

const search = shallowRef('')
const page = shallowRef(1)
const pageSize = 5

const filteredMembers = computed(() => {
  const query = normalizeSearch(search.value)
  if (!query) {
    return props.members
  }
  return props.members.filter((member) =>
    memberSearchText(member).includes(query),
  )
})
const pageCount = computed(() =>
  Math.max(1, Math.ceil(filteredMembers.value.length / pageSize)),
)
const pagedMembers = computed(() => {
  const start = (page.value - 1) * pageSize
  return filteredMembers.value.slice(start, start + pageSize)
})
const resultLabel = computed(() => {
  const count = filteredMembers.value.length
  if (count === props.members.length) {
    return count === 1 ? '1 member' : `${count} members`
  }
  return count === 1 ? '1 match' : `${count} matches`
})

watch([search, () => props.members], () => {
  page.value = 1
})

watch(
  () => props.active,
  (active) => {
    if (!active) {
      return
    }
    search.value = ''
    page.value = 1
  },
)

watch(pageCount, (nextPageCount) => {
  if (page.value > nextPageCount) {
    page.value = nextPageCount
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
  return normalizeSearch(
    [
      displayMember(member),
      memberSubtitle(member),
      member.email,
      member.preferredUsername,
      member.role,
      member.type,
      member.id,
    ].join(' '),
  )
}

function normalizeSearch(value: string) {
  return value.trim().toLowerCase()
}
</script>

<template>
  <div class="admin-group-members-list">
    <div class="admin-group-members-list__toolbar">
      <v-text-field
        v-model="search"
        label="Search members"
        prepend-inner-icon="mdi-magnify"
        variant="outlined"
        density="compact"
        clearable
        hide-details
      />
      <span class="admin-group-members-list__count">{{ resultLabel }}</span>
    </div>

    <v-list
      v-if="pagedMembers.length"
      bg-color="transparent"
      lines="two"
      class="admin-group-members-list__list"
    >
      <v-list-item
        v-for="member in pagedMembers"
        :key="member.id"
        :title="displayMember(member)"
        :subtitle="memberSubtitle(member)"
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
      v-else-if="props.members.length"
      compact
      icon="mdi-account-search-outline"
      title="No matching members"
      text="Try another name, email, or identifier."
    />

    <AppEmptyState
      v-else
      compact
      icon="mdi-account-multiple-remove-outline"
      title="No members"
      text="Add a user or service account before this group grants access."
    />

    <div v-if="pageCount > 1" class="admin-group-members-list__pagination">
      <v-pagination
        v-model="page"
        :length="pageCount"
        density="comfortable"
        rounded="circle"
        total-visible="5"
      />
    </div>
  </div>
</template>

<style scoped>
.admin-group-members-list {
  display: grid;
  gap: 12px;
}

.admin-group-members-list__toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
}

.admin-group-members-list__count {
  color: rgb(var(--v-theme-on-surface-variant));
  font-size: 0.875rem;
  white-space: nowrap;
}

.admin-group-members-list__list {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  border-radius: 8px;
}

.admin-group-members-list__pagination {
  display: flex;
  justify-content: center;
}

@media (max-width: 720px) {
  .admin-group-members-list__toolbar {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
