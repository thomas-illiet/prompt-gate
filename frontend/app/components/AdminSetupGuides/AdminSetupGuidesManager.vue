<script setup lang="ts">
import AdminSetupGuideDialog from './AdminSetupGuideDialog.vue'
import type { SetupGuide, SetupGuidePayload } from '~/types/setup-guides'
const admin = useAdminSetupGuides()
const dialogOpen = shallowRef(false)
const deleteTarget = shallowRef<SetupGuide | null>(null)
function create() {
  admin.selectedGuide.value = null
  dialogOpen.value = true
}
function edit(guide: SetupGuide) {
  admin.selectedGuide.value = guide
  dialogOpen.value = true
}
async function save(payload: SetupGuidePayload) {
  await admin.save(payload)
  dialogOpen.value = false
}
async function remove() {
  if (!deleteTarget.value) return
  await admin.remove(deleteTarget.value.id)
  deleteTarget.value = null
}
</script>
<template>
  <v-row>
    <v-col cols="12"
      ><AppPageHero
        icon="mdi-book-cog-outline"
        kicker="Documentation"
        title="Setup guides"
        copy="Configure the client guides displayed to PromptGate users."
        stat-label="Guides"
        :stat-value="String(admin.guides.value.length)"
      /></v-col
    >
    <v-col v-if="admin.error.value" cols="12"
      ><v-alert type="error">{{ admin.error.value }}</v-alert></v-col
    >
    <v-col cols="12"
      ><AppSectionCard
        icon="mdi-format-list-numbered"
        title="Client guides"
        subtitle="Global display order and content"
        ><template #actions
          ><v-btn color="primary" prepend-icon="mdi-plus" @click="create"
            >Create guide</v-btn
          ></template
        ><AdminSetupGuidesTable
          :guides="admin.guides.value"
          :loading="admin.loading.value"
          @edit="edit"
          @remove="deleteTarget = $event"
          @reorder="admin.reorder" /></AppSectionCard
    ></v-col>
  </v-row>
  <AdminSetupGuideDialog
    v-model="dialogOpen"
    :guide="admin.selectedGuide.value"
    :next-position="admin.guides.value.length"
    :saving="admin.saving.value"
    @save="save"
  />
  <v-dialog
    :model-value="!!deleteTarget"
    max-width="480"
    @update:model-value="!$event && (deleteTarget = null)"
    ><v-card
      ><v-card-title>Delete setup guide?</v-card-title
      ><v-card-text
        >This permanently removes “{{ deleteTarget?.title }}”.</v-card-text
      ><v-card-actions
        ><v-spacer /><v-btn @click="deleteTarget = null">Cancel</v-btn
        ><v-btn color="error" @click="remove">Delete</v-btn></v-card-actions
      ></v-card
    ></v-dialog
  >
</template>
