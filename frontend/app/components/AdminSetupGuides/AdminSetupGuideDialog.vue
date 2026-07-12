<script setup lang="ts">
import type { SetupGuide, SetupGuidePayload } from '~/types/setup-guides'
import {
  SETUP_GUIDE_VARIABLES,
  validateSetupGuideTemplate,
} from '~/utils/setup-guide-template'

const props = defineProps<{
  guide: SetupGuide | null
  saving: boolean
  nextPosition: number
}>()
const emit = defineEmits<{ save: [payload: SetupGuidePayload] }>()
const open = defineModel<boolean>({ required: true })
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
const templateError = computed(() => validateSetupGuideTemplate(form.template))
const variableHelp = SETUP_GUIDE_VARIABLES.map((value) => `{{${value}}}`).join(
  ', ',
)
const loopHelp = '{{#models}}...{{/models}}'

watch(open, (isOpen) => {
  if (!isOpen) return
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
          filePaths: [...source.filePaths],
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
function submit() {
  if (templateError.value) return
  emit('save', {
    ...form,
    filePaths: filePathsText.value
      .split('\n')
      .map((v) => v.trim())
      .filter(Boolean),
  })
}
</script>

<template>
  <v-dialog v-model="open" max-width="1100" scrollable>
    <v-card rounded="lg">
      <v-card-title>{{
        guide ? 'Edit setup guide' : 'Create setup guide'
      }}</v-card-title>
      <v-card-text
        ><v-row>
          <v-col cols="12" md="6"
            ><v-text-field
              v-model="form.identifier"
              label="Identifier"
              hint="lowercase-kebab-case" /><v-text-field
              v-model="form.title"
              label="Title" /><v-text-field
              v-model="form.subtitle"
              label="Subtitle" /><v-text-field
              v-model="form.icon"
              label="Material Design icon" /><v-select
              v-model="form.compatibility"
              label="Provider compatibility"
              :items="['openai', 'anthropic', 'both']" /><v-select
              v-model="form.modelMode"
              label="Model selection"
              :items="['single', 'all', 'none']" /><v-textarea
              v-model="filePathsText"
              label="File paths (one per line)"
              rows="2" /><v-switch
              v-model="form.enabled"
              label="Enabled"
              color="primary"
          /></v-col>
          <v-col cols="12" md="6"
            ><v-textarea
              v-model="form.template"
              label="Template"
              rows="14"
              :error-messages="templateError ? [templateError] : []" />
            <div class="text-caption mb-3">
              Variables: {{ variableHelp }}. Loop: {{ loopHelp }}
            </div>
            <AdminSetupGuidePreview :guide="form"
          /></v-col> </v-row
      ></v-card-text>
      <v-card-actions
        ><v-spacer /><v-btn @click="open = false">Cancel</v-btn
        ><v-btn
          color="primary"
          :disabled="!!templateError || !form.identifier || !form.title"
          :loading="saving"
          @click="submit"
          >Save</v-btn
        ></v-card-actions
      >
    </v-card>
  </v-dialog>
</template>
