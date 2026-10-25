<script setup lang="ts">
import AdminSetupGuideDialog from './AdminSetupGuideDialog.vue'
import type { SetupGuide, SetupGuidePayload } from '~/types/setup-guides'

const admin = useAdminSetupGuides()
const dialogOpen = shallowRef(false)
const deletePending = shallowRef(false)
const deleteDialog = useTargetDialog<SetupGuide>()

const guideSummary = computed(() => {
  if (admin.loading.value && admin.guides.value.length === 0) return '…'

  const total = admin.guides.value.length
  const enabled = admin.guides.value.filter((guide) => guide.enabled).length
  return `${enabled}/${total} enabled`
})

function createGuide() {
  admin.selectedGuide.value = null
  dialogOpen.value = true
}

function editGuide(guide: SetupGuide) {
  admin.selectedGuide.value = guide
  dialogOpen.value = true
}

async function saveGuide(payload: SetupGuidePayload) {
  await admin.save(payload)
  dialogOpen.value = false
}

async function confirmDelete() {
  if (!deleteDialog.target.value) return

  deletePending.value = true
  try {
    await admin.remove(deleteDialog.target.value.id)
    deleteDialog.close()
  } finally {
    deletePending.value = false
  }
}
</script>

<template>
  <v-row>
    <v-col cols="12">
      <AppPageHeader
        icon="mdi-book-cog-outline"
        kicker="Documentation"
        title="Setup guides"
        copy="Build the launch paths users follow from their first token to a working client."
        visual="book"
        stat-label="Guide status"
        :stat-busy="admin.loading.value"
        :stat-value="guideSummary"
      />
    </v-col>

    <v-col v-if="admin.error.value" cols="12">
      <v-alert type="warning" variant="tonal" rounded="lg">
        {{ admin.error.value }}
      </v-alert>
    </v-col>

    <v-col cols="12">
      <AppSectionCard
        icon="mdi-format-list-numbered"
        title="Client guides"
        subtitle="Shape the global display order and the instructions users can launch."
      >
        <template #actions>
          <v-btn
            color="primary"
            prepend-icon="mdi-plus"
            rounded="xl"
            @click="createGuide"
          >
            Create guide
          </v-btn>
          <v-btn
            color="primary"
            prepend-icon="mdi-refresh"
            rounded="xl"
            variant="tonal"
            :loading="admin.loading.value"
            @click="admin.reload"
          >
            Refresh
          </v-btn>
        </template>

        <AdminSetupGuidesTable
          :guides="admin.guides.value"
          :loading="admin.loading.value"
          @edit="editGuide"
          @remove="deleteDialog.open"
          @reorder="admin.reorder"
        />
      </AppSectionCard>
    </v-col>
  </v-row>

  <AdminSetupGuideDialog
    v-model="dialogOpen"
    :guide="admin.selectedGuide.value"
    :next-position="admin.guides.value.length"
    :saving="admin.saving.value"
    @save="saveGuide"
  />

  <AppConfirmDialog
    v-model="deleteDialog.isOpen.value"
    confirm-color="error"
    confirm-label="Delete guide"
    icon="mdi-delete-alert-outline"
    :loading="deletePending"
    :message="
      deleteDialog.target.value
        ? `Permanently delete “${deleteDialog.target.value.title}”?`
        : 'Permanently delete this setup guide?'
    "
    title="Delete setup guide?"
    @cancel="deleteDialog.close"
    @confirm="confirmDelete"
  />
</template>
