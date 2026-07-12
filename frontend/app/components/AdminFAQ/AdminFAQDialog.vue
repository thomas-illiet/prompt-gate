<script setup lang="ts">
import type { FAQEntry, FAQPayload } from '~/types/faq'

const props = defineProps<{ entry: FAQEntry | null; loading: boolean; previewing: boolean; preview: (markdown: string) => Promise<{ renderedHtml: string }> }>()
const emit = defineEmits<{ save: [payload: FAQPayload] }>()
const isOpen = defineModel<boolean>({ default: false })
const question = shallowRef('')
const answer = shallowRef('')
const published = shallowRef(false)
const tab = shallowRef<'edit' | 'preview'>('edit')
const previewHtml = shallowRef('')
const submitted = shallowRef(false)
const initial = shallowRef('')
const discardOpen = shallowRef(false)
const formId = useId()
const snapshot = computed(() => JSON.stringify({ question: question.value, answer: answer.value, published: published.value }))
const dirty = computed(() => snapshot.value !== initial.value)
const questionError = computed(() => submitted.value && !question.value.trim() ? 'Question is required.' : submitted.value && [...question.value.trim()].length > 300 ? 'Use at most 300 characters.' : '')
const answerError = computed(() => submitted.value && !answer.value.trim() ? 'Answer is required.' : '')

watch([isOpen, () => props.entry], ([open]) => {
  if (!open) return
  question.value = props.entry?.question ?? ''
  answer.value = props.entry?.answer ?? ''
  published.value = props.entry?.published ?? false
  previewHtml.value = props.entry?.renderedHtml ?? ''
  tab.value = 'edit'
  submitted.value = false
  nextTick(() => { initial.value = snapshot.value })
}, { immediate: true })

watch(tab, async (value) => {
  if (value !== 'preview') return
  const result = await props.preview(answer.value)
  previewHtml.value = result.renderedHtml
})

function close() {
  if (dirty.value) { discardOpen.value = true; return }
  isOpen.value = false
}
function discard() { discardOpen.value = false; isOpen.value = false }
function save() {
  submitted.value = true
  if (questionError.value || answerError.value) return
  emit('save', { question: question.value.trim(), answer: answer.value.trim(), published: published.value })
}
</script>

<template>
  <AppDialogCard v-model="isOpen" icon="mdi-text-box-edit-outline" :loading="props.loading" max-width="900" persistent subtitle="Write the answer in Markdown and verify the exact user-facing render before publishing." :title="props.entry ? 'Edit FAQ entry' : 'Create FAQ entry'">
    <v-tabs v-model="tab" class="mb-4"><v-tab value="edit">Edit</v-tab><v-tab value="preview">Preview</v-tab></v-tabs>
    <form v-show="tab === 'edit'" :id="formId" @submit.prevent="save">
      <v-text-field v-model="question" counter="300" label="Question" variant="outlined" :error-messages="questionError ? [questionError] : []" />
      <v-textarea v-model="answer" auto-grow label="Answer (Markdown)" rows="12" variant="outlined" :error-messages="answerError ? [answerError] : []" />
      <v-switch v-model="published" color="primary" hide-details label="Published" />
    </form>
    <AdminFAQPreview v-show="tab === 'preview'" :html="previewHtml" :loading="props.previewing" />
    <template #actions>
      <AppDialogCloseButton :disabled="props.loading" label="Cancel" @click="close" />
      <AppDialogActionButton color="primary" :form="formId" :label="props.entry ? 'Save entry' : 'Create entry'" :loading="props.loading" type="submit" />
    </template>
  </AppDialogCard>
  <AppConfirmDialog v-model="discardOpen" confirm-color="warning" confirm-label="Discard changes" icon="mdi-alert-outline" message="Your unsaved FAQ changes will be lost." title="Discard unsaved changes?" @cancel="discardOpen = false" @confirm="discard" />
</template>
