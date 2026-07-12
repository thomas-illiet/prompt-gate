<script setup lang="ts">
import type {
  SetupGuideCompatibility,
  SetupGuideModelMode,
} from '~/types/setup-guides'

const props = defineProps<{
  identifierError: string
  titleError: string
  iconError: string
}>()
const identifier = defineModel<string>('identifier', { required: true })
const title = defineModel<string>('title', { required: true })
const subtitle = defineModel<string>('subtitle', { required: true })
const icon = defineModel<string>('icon', { required: true })
const compatibility = defineModel<SetupGuideCompatibility>('compatibility', {
  required: true,
})
const modelMode = defineModel<SetupGuideModelMode>('modelMode', {
  required: true,
})
const enabled = defineModel<boolean>('enabled', { required: true })

const compatibilityOptions = [
  { title: 'OpenAI compatible', value: 'openai' },
  { title: 'Anthropic compatible', value: 'anthropic' },
  { title: 'Both provider types', value: 'both' },
]
const modelModeOptions = [
  { title: 'One selected model', value: 'single' },
  { title: 'All available models', value: 'all' },
  { title: 'No model required', value: 'none' },
]
</script>

<template>
  <section class="setup-guide-settings" aria-labelledby="guide-settings-title">
    <div class="setup-guide-settings__heading">
      <div>
        <h3 id="guide-settings-title">Guide settings</h3>
        <p>Choose its label, audience and model behavior.</p>
      </div>
      <v-switch
        v-model="enabled"
        label="Published"
        color="success"
        hide-details
        inset
      />
    </div>

    <v-row>
      <v-col cols="12" md="5">
        <v-text-field
          v-model="title"
          label="Title"
          placeholder="Python SDK"
          variant="outlined"
          density="comfortable"
          :error-messages="props.titleError ? [props.titleError] : []"
        />
      </v-col>
      <v-col cols="12" md="7">
        <v-text-field
          v-model="subtitle"
          label="Short description"
          placeholder="Configure the Python SDK for PromptGate."
          variant="outlined"
          density="comfortable"
        />
      </v-col>
      <v-col cols="12" md="6">
        <v-text-field
          v-model="identifier"
          label="Identifier"
          placeholder="python-sdk"
          variant="outlined"
          density="comfortable"
          hint="Stable lowercase key used internally"
          persistent-hint
          :error-messages="
            props.identifierError ? [props.identifierError] : []
          "
        />
      </v-col>
      <v-col cols="12" md="6">
        <v-text-field
          v-model="icon"
          label="Icon"
          placeholder="mdi-language-python"
          variant="outlined"
          density="comfortable"
          :prepend-inner-icon="icon || 'mdi-shape-outline'"
          :error-messages="props.iconError ? [props.iconError] : []"
        />
      </v-col>
      <v-col cols="12" md="6">
        <v-select
          v-model="compatibility"
          :items="compatibilityOptions"
          label="Provider compatibility"
          variant="outlined"
          density="comfortable"
        />
      </v-col>
      <v-col cols="12" md="6">
        <v-select
          v-model="modelMode"
          :items="modelModeOptions"
          label="Model behavior"
          variant="outlined"
          density="comfortable"
        />
      </v-col>
    </v-row>
  </section>
</template>

<style scoped>
.setup-guide-settings__heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 18px;
}

.setup-guide-settings__heading h3,
.setup-guide-settings__heading p {
  margin: 0;
}

.setup-guide-settings__heading p {
  margin-top: 4px;
  color: rgb(var(--app-shell-text-secondary));
}

@media (max-width: 600px) {
  .setup-guide-settings__heading {
    flex-direction: column;
    gap: 8px;
  }
}
</style>
