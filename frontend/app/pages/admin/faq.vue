<script setup lang="ts">
import type { FAQEntry, FAQPayload } from '~/types/faq'

definePageMeta({ requiredRoles: ['admin'], title: 'FAQ', icon: 'mdi-frequently-asked-questions', drawerIndex: 1, drawerSection: 'Content' })

const faq = useAdminFAQ()
const dialogOpen = shallowRef(false)
const deleteDialog = useTargetDialog<FAQEntry>()
const publishedCount = computed(() => faq.entries.value.filter((entry) => entry.published).length)

function createEntry() { faq.selectedEntry.value = null; dialogOpen.value = true }
function editEntry(entry: FAQEntry) { faq.selectedEntry.value = entry; dialogOpen.value = true }
async function saveEntry(payload: FAQPayload) {
  if (faq.selectedEntry.value) await faq.update(faq.selectedEntry.value.id, payload)
  else await faq.create(payload)
  dialogOpen.value = false
}
async function toggleEntry(entry: FAQEntry) { await faq.update(entry.id, { question: entry.question, answer: entry.answer, published: !entry.published }) }
async function confirmDelete() {
  if (!deleteDialog.target.value) return
  await faq.remove(deleteDialog.target.value.id)
  deleteDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12"><AppPageHero icon="mdi-frequently-asked-questions" kicker="Documentation" title="FAQ management" copy="Maintain Markdown answers, preview their final rendering, and control publication." stat-label="Published on page" :stat-value="`${publishedCount}/${faq.total.value}`" /></v-col>
      <v-col v-if="faq.listError.value" cols="12"><v-alert type="warning" variant="tonal" rounded="lg">{{ faq.listError.value }}</v-alert></v-col>
      <v-col cols="12"><AdminFAQTable :items="faq.entries.value" :loading="faq.loading.value" :page="faq.page.value" :page-size="faq.pageSize.value" :sort-by="faq.sortBy.value" :sort-dir="faq.sortDir.value" :total="faq.total.value" @create="createEntry" @delete="deleteDialog.open" @edit="editEntry" @move="faq.move" @refresh="faq.reload" @toggle="toggleEntry" @update:page="faq.setPage" @update:page-size="faq.setPageSize" @update:sort="faq.setSort" /></v-col>
    </v-row>
    <AdminFAQDialog v-model="dialogOpen" :entry="faq.selectedEntry.value" :loading="faq.saving.value" :preview="faq.preview" :previewing="faq.previewing.value" @save="saveEntry" />
    <AppConfirmDialog v-model="deleteDialog.isOpen.value" confirm-color="error" confirm-label="Delete entry" icon="mdi-delete-alert-outline" :loading="faq.saving.value" :message="deleteDialog.target.value ? `Delete “${deleteDialog.target.value.question}”?` : 'Delete this FAQ entry?'" title="Delete FAQ entry" @cancel="deleteDialog.close" @confirm="confirmDelete" />
  </v-container>
</template>
