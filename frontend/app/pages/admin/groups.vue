<script setup lang="ts">
import AdminGroupDialog from '~/components/AdminGroups/AdminGroupDialog.vue'
import AdminGroupMembersDialog from '~/components/AdminGroups/AdminGroupMembersDialog.vue'
import AdminGroupsFilters from '~/components/AdminGroups/AdminGroupsFilters.vue'
import AdminGroupsTable from '~/components/AdminGroups/AdminGroupsTable.vue'
import type {
  AccessGroup,
  CreateGroupPayload,
  GroupModelPatternValidationPayload,
  UpdateGroupPayload,
} from '~/types/groups'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Groups',
  icon: 'mdi-account-multiple-check-outline',
  drawerIndex: 3,
  drawerSection: 'Identities',
})

const adminGroups = useAdminGroups()
const groupDialogOpen = shallowRef(false)
const membersDialogOpen = shallowRef(false)
const deleteDialog = useTargetDialog<AccessGroup>()

const totalLabel = computed(() => {
  const total = adminGroups.total.value
  return total === 1 ? '1 configured' : `${total} configured`
})

async function openCreateDialog() {
  adminGroups.selectedGroup.value = null
  adminGroups.clearModelValidation()
  await adminGroups.loadProviderOptions()
  groupDialogOpen.value = true
}

async function openEditDialog(group: AccessGroup) {
  adminGroups.clearModelValidation()
  await Promise.all([
    adminGroups.loadProviderOptions(),
    adminGroups.loadGroup(group.id),
  ])
  groupDialogOpen.value = true
}

async function saveGroup(payload: CreateGroupPayload | UpdateGroupPayload) {
  if (adminGroups.selectedGroup.value) {
    await adminGroups.updateGroup(adminGroups.selectedGroup.value.id, {
      displayName: payload.displayName,
      description: payload.description,
      providerIds: payload.providerIds,
      modelPatterns: payload.modelPatterns,
      excludedModelPatterns: payload.excludedModelPatterns,
    })
  } else if ('name' in payload) {
    await adminGroups.createGroup(payload)
  } else {
    return
  }
  groupDialogOpen.value = false
  adminGroups.clearModelValidation()
}

async function validateModelPatterns(
  payload: GroupModelPatternValidationPayload,
) {
  try {
    await adminGroups.validateModelPatterns(payload)
  } catch {
    // The composable exposes the user-facing validation error.
  }
}

async function openMembersDialog(group: AccessGroup) {
  await Promise.all([
    adminGroups.loadMemberOptions(),
    adminGroups.loadGroup(group.id),
  ])
  membersDialogOpen.value = true
}

async function refreshMembers() {
  if (!adminGroups.selectedGroup.value) {
    return
  }
  await Promise.all([
    adminGroups.loadMemberOptions(),
    adminGroups.loadGroup(adminGroups.selectedGroup.value.id),
  ])
}

async function addMember(userId: string) {
  if (!adminGroups.selectedGroup.value) {
    return
  }
  await adminGroups.addMember(adminGroups.selectedGroup.value.id, userId)
}

async function removeMember(userId: string) {
  if (!adminGroups.selectedGroup.value) {
    return
  }
  await adminGroups.removeMember(adminGroups.selectedGroup.value.id, userId)
}

async function confirmDelete() {
  if (!deleteDialog.target.value) {
    return
  }
  await adminGroups.deleteGroup(deleteDialog.target.value.id)
  deleteDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-account-multiple-check-outline"
          kicker="Proxy authorization"
          title="Groups"
          copy="Grant proxy access by assigning users and service accounts to provider and model rules."
          stat-label="Access groups"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <AdminGroupsFilters
          :search="adminGroups.search.value"
          @update:search="adminGroups.setSearch"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="adminGroups.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminGroups.listError.value }}
        </v-alert>

        <AdminGroupsTable
          :items="adminGroups.groups.value"
          :loading="adminGroups.loading.value"
          :page="adminGroups.page.value"
          :page-size="adminGroups.pageSize.value"
          :sort-by="adminGroups.sortBy.value"
          :sort-dir="adminGroups.sortDir.value"
          :total="adminGroups.total.value"
          @create="openCreateDialog"
          @delete="deleteDialog.open"
          @edit="openEditDialog"
          @manage-members="openMembersDialog"
          @refresh="adminGroups.reload"
          @update:page="adminGroups.setPage"
          @update:page-size="adminGroups.setPageSize"
          @update:sort="adminGroups.setSort"
        />
      </v-col>
    </v-row>

    <AdminGroupDialog
      v-model="groupDialogOpen"
      :group="adminGroups.selectedGroup.value"
      :loading="adminGroups.saving.value"
      :model-validation="adminGroups.modelValidation.value"
      :model-validation-error="adminGroups.modelValidationError.value"
      :model-validation-loading="adminGroups.modelValidationLoading.value"
      :providers="adminGroups.providerOptions.value"
      @clear-model-validation="adminGroups.clearModelValidation"
      @save="saveGroup"
      @validate-models="validateModelPatterns"
    />

    <AdminGroupMembersDialog
      v-model="membersDialogOpen"
      :group="adminGroups.selectedGroup.value"
      :loading="adminGroups.saving.value"
      :member-options="adminGroups.memberOptions.value"
      :options-loading="adminGroups.memberLoading.value"
      @add="addMember"
      @refresh="refreshMembers"
      @remove="removeMember"
    />

    <AppConfirmDialog
      v-model="deleteDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Delete group"
      icon="mdi-delete-alert-outline"
      :loading="adminGroups.saving.value"
      :message="
        deleteDialog.target.value
          ? `Delete group ${deleteDialog.target.value.name}.`
          : 'Delete this group.'
      "
      title="Delete group"
      @cancel="deleteDialog.close"
      @confirm="confirmDelete"
    />
  </v-container>
</template>
