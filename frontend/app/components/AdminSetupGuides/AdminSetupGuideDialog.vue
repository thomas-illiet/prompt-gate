<script setup lang="ts">
import AdminSetupGuideContentStep from './AdminSetupGuideContentStep.vue'
import AdminSetupGuideSettingsStep from './AdminSetupGuideSettingsStep.vue'
import type { SetupGuide, SetupGuidePayload } from '~/types/setup-guides'
import { validateSetupGuideTemplate } from '~/utils/setup-guide-template'

type DialogStep = 1 | 2

const props = defineProps<{
  guide: SetupGuide | null
  saving: boolean
  nextPosition: number
}>()
const emit = defineEmits<{ save: [payload: SetupGuidePayload] }>()
const open = defineModel<boolean>({ required: true })
const step = shallowRef<DialogStep>(1)
const form = reactive<SetupGuidePayload>({
  identifier: '',
  title: '',
  subtitle: '',
  icon: 'mdi-file-code-outline',
  compatibility: 'openai',
  modelMode: 'single',
  filePaths: [],
  template: '',
  enabled: true,
  position: 0,
})
const filePathsText = shallowRef('')
const identifierError = computed(() => {
  if (!form.identifier.trim()) return 'Identifier is required.'
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(form.identifier))
    return 'Use lowercase kebab-case.'
  return ''
})
const titleError = computed(() =>
  form.title.trim() ? '' : 'Title is required.',
)
const iconError = computed(() =>
  /^mdi-[a-z0-9-]+$/.test(form.icon) ? '' : 'Use an mdi-* icon name.',
)
const settingsValid = computed(
  () => !identifierError.value && !titleError.value && !iconError.value,
)
const templateError = computed(() => validateSetupGuideTemplate(form.template))
const dialogTitle = computed(() =>
  props.guide ? 'Edit setup guide' : 'Create setup guide',
)
const dialogSubtitle = computed(() =>
  step.value === 1
    ? 'Define how this guide appears and which providers it supports.'
    : 'Write the guide content and check the rendered result.',
)

watch(open, (isOpen) => {
  if (!isOpen) return
  step.value = 1
  const source = props.guide
  Object.assign(
    form,
    source
      ? {
          identifier: source.identifier,
          title: source.title,
          subtitle: source.subtitle,
          icon: source.icon,
          compatibility: source.compatibility,
          modelMode: source.modelMode,
          filePaths: Array.isArray(source.filePaths) ? [...source.filePaths] : [],
          template: source.template,
          enabled: source.enabled,
          position: source.position,
        }
      : {
          identifier: '',
          title: '',
          subtitle: '',
          icon: 'mdi-file-code-outline',
          compatibility: 'openai',
          modelMode: 'single',
          filePaths: [],
          template: '',
          enabled: true,
          position: props.nextPosition,
        },
  )
  filePathsText.value = form.filePaths.join('\n')
})

function goToContent() {
  if (settingsValid.value) step.value = 2
}

function submit() {
  if (!settingsValid.value || templateError.value) return
  emit('save', {
    ...form,
    filePaths: filePathsText.value
      .split('\n')
      .map((value) => value.trim())
      .filter(Boolean),
  })
}
</script>

<template>
  <AppDialogCard
    v-model="open"
    icon="mdi-book-edit-outline"
    :loading="props.saving"
    max-width="1040"
    :subtitle="dialogSubtitle"
    :title="dialogTitle"
  >
    <nav class="setup-guide-steps" aria-label="Setup guide form steps">
      <button
        type="button"
        class="setup-guide-step"
        :class="{ 'setup-guide-step--active': step === 1 }"
        :aria-current="step === 1 ? 'step' : undefined"
        @click="step = 1"
      >
        <span>1</span>
        <span><strong>Settings</strong><small>Identity and scope</small></span>
      </button>
      <div class="setup-guide-steps__line" />
      <button
        type="button"
        class="setup-guide-step"
        :class="{ 'setup-guide-step--active': step === 2 }"
        :aria-current="step === 2 ? 'step' : undefined"
        :disabled="!settingsValid"
        @click="goToContent"
      >
        <span>2</span>
        <span><strong>Content</strong><small>Template and preview</small></span>
      </button>
    </nav>

    <AdminSetupGuideSettingsStep
      v-if="step === 1"
      v-model:identifier="form.identifier"
      v-model:title="form.title"
      v-model:subtitle="form.subtitle"
      v-model:icon="form.icon"
      v-model:compatibility="form.compatibility"
      v-model:model-mode="form.modelMode"
      v-model:enabled="form.enabled"
      :identifier-error="identifierError"
      :title-error="titleError"
      :icon-error="iconError"
    />
    <AdminSetupGuideContentStep
      v-else
      v-model:file-paths-text="filePathsText"
      v-model:template="form.template"
      :guide="form"
      :template-error="templateError"
    />

    <template #actions>
      <AppDialogCloseButton
        :disabled="props.saving"
        label="Cancel"
        @click="open = false"
      />
      <AppDialogActionButton
        v-if="step === 1"
        label="Continue"
        :disabled="!settingsValid"
        prepend-icon="mdi-arrow-right"
        @click="goToContent"
      />
      <template v-else>
        <AppDialogActionButton
          label="Back"
          variant="tonal"
          prepend-icon="mdi-arrow-left"
          :disabled="props.saving"
          @click="step = 1"
        />
        <AppDialogActionButton
          :label="props.guide ? 'Save changes' : 'Create guide'"
          :loading="props.saving"
          :disabled="Boolean(templateError)"
          prepend-icon="mdi-content-save-outline"
          @click="submit"
        />
      </template>
    </template>
  </AppDialogCard>
</template>

<style scoped>
.setup-guide-steps {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 48px minmax(0, 1fr);
  align-items: center;
  margin-bottom: 24px;
  padding: 12px;
  border: 1px solid rgba(var(--app-shell-border), 0.5);
  border-radius: 14px;
  background: rgba(var(--app-shell-surface-muted), 0.52);
}

.setup-guide-step {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 8px;
  border: 0;
  border-radius: 10px;
  color: rgb(var(--app-shell-text-secondary));
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.setup-guide-step:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.setup-guide-step > span:first-child {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  border-radius: 50%;
  background: rgba(var(--v-theme-primary), 0.1);
  font-weight: 700;
}

.setup-guide-step > span:last-child {
  display: grid;
}

.setup-guide-step small {
  margin-top: 2px;
  color: rgb(var(--app-shell-text-muted));
}

.setup-guide-step--active {
  color: rgb(var(--v-theme-primary));
  background: rgba(var(--v-theme-primary), 0.08);
}

.setup-guide-step--active > span:first-child {
  color: rgb(var(--v-theme-on-primary));
  background: rgb(var(--v-theme-primary));
}

.setup-guide-steps__line {
  height: 1px;
  background: rgba(var(--app-shell-border), 0.8);
}

@media (max-width: 600px) {
  .setup-guide-steps {
    grid-template-columns: 1fr;
  }

  .setup-guide-steps__line {
    display: none;
  }
}
</style>
