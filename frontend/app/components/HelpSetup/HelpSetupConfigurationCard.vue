<script setup lang="ts">
import type { HelpSetupProvider } from '~/types/user-service'
import { ANTHROPIC_MODEL_PLACEHOLDER, providerLabel } from '~/utils/help-setup'

const ALL_MODELS_LABEL = 'All'

const props = defineProps<{
  modelOptions: string[]
  modelSelectMode: 'all' | 'none' | 'single'
  providers: HelpSetupProvider[]
  selectedProvider: HelpSetupProvider | null
}>()

const selectedProviderName = defineModel<string>('providerName', {
  required: true,
})
const selectedModel = defineModel<string>('model', { required: true })

const providerOptions = computed(() =>
  props.providers.map((provider) => ({
    icon: providerIcon(provider.type),
    modelsLabel:
      provider.type === 'anthropic'
        ? ANTHROPIC_MODEL_PLACEHOLDER
        : `${provider.models.length} models`,
    title: providerLabel(provider),
    type: provider.type,
    value: provider.name,
  })),
)

const selectedProviderOption = computed(
  () =>
    providerOptions.value.find(
      (provider) => provider.value === selectedProviderName.value,
    ) ?? null,
)
const modelSelectItems = computed(() => {
  if (props.modelSelectMode === 'all') {
    return [ALL_MODELS_LABEL]
  }

  if (props.modelSelectMode === 'none') {
    return [ANTHROPIC_MODEL_PLACEHOLDER]
  }

  return props.modelOptions
})
const displayedModel = computed({
  get() {
    if (props.modelSelectMode === 'all') {
      return ALL_MODELS_LABEL
    }

    if (props.modelSelectMode === 'none') {
      return ANTHROPIC_MODEL_PLACEHOLDER
    }

    return selectedModel.value
  },
  set(value: string) {
    if (props.modelSelectMode === 'single') {
      selectedModel.value = value
    }
  },
})
const modelSelectDisabled = computed(() => props.modelSelectMode !== 'single')
const modelSelectHint = computed(() => {
  if (props.modelSelectMode === 'all') {
    return 'This documentation includes every model from the provider.'
  }

  if (props.modelSelectMode === 'none') {
    return 'Claude Code uses the endpoint directly.'
  }

  return ''
})
const showModelError = computed(
  () =>
    props.modelSelectMode === 'single' &&
    props.selectedProvider?.type !== 'anthropic' &&
    Boolean(props.selectedProvider?.modelsError),
)

// providerIcon returns the icon used for a provider type.
function providerIcon(type: HelpSetupProvider['type']) {
  switch (type) {
    case 'anthropic':
      return 'mdi-alpha-a-circle-outline'
    case 'ollama':
      return 'mdi-layers-triple-outline'
    default:
      return 'mdi-robot-outline'
  }
}
</script>

<template>
  <AppSectionCard
    icon="mdi-tune-variant"
    title="Configuration"
    subtitle="Choose a provider and model. Client snippets update automatically."
  >
    <div class="help-setup-configuration-card__setup">
      <div class="help-setup-configuration-card__field">
        <div class="help-setup-configuration-card__label">Provider</div>
        <v-select
          v-model="selectedProviderName"
          bg-color="surface"
          class="help-setup-configuration-card__provider-select"
          density="comfortable"
          hide-details
          item-title="title"
          item-value="value"
          :items="providerOptions"
          menu-icon="mdi-chevron-down"
          prepend-inner-icon="mdi-server-network"
          rounded="lg"
          variant="solo-filled"
        >
          <template #selection="{ item }">
            <div class="help-setup-configuration-card__provider-selection">
              <v-icon :icon="item.icon" size="18" />
              <span>{{ item.title }}</span>
              <span class="help-setup-configuration-card__provider-type">
                {{ item.type }}
              </span>
            </div>
          </template>

          <template #item="{ props: itemProps, item }">
            <v-list-item v-bind="itemProps" :prepend-icon="item.icon">
              <template #subtitle>
                <span
                  class="help-setup-configuration-card__provider-option-subtitle"
                >
                  {{ item.type }} · {{ item.modelsLabel }}
                </span>
              </template>
            </v-list-item>
          </template>
        </v-select>

        <div
          v-if="selectedProviderOption"
          class="help-setup-configuration-card__provider-meta"
        >
          <v-chip
            :prepend-icon="selectedProviderOption.icon"
            rounded="lg"
            size="small"
            variant="tonal"
          >
            {{ selectedProviderOption.type }}
          </v-chip>
          <v-chip
            prepend-icon="mdi-cube-outline"
            rounded="lg"
            size="small"
            variant="tonal"
          >
            {{ selectedProviderOption.modelsLabel }}
          </v-chip>
        </div>
      </div>

      <div class="help-setup-configuration-card__field">
        <div class="help-setup-configuration-card__label">Model</div>
        <v-select
          v-model="displayedModel"
          bg-color="surface"
          density="comfortable"
          :disabled="modelSelectDisabled"
          hide-details
          :items="modelSelectItems"
          menu-icon="mdi-chevron-down"
          placeholder="Select a model"
          prepend-inner-icon="mdi-cube-outline"
          rounded="lg"
          variant="solo-filled"
        />
        <div v-if="modelSelectHint" class="help-setup-configuration-card__hint">
          {{ modelSelectHint }}
        </div>
      </div>
    </div>

    <v-alert
      v-if="showModelError"
      class="ma-4 mt-0"
      density="comfortable"
      icon="mdi-alert-circle-outline"
      type="warning"
      variant="tonal"
    >
      Models unavailable for this provider:
      {{ props.selectedProvider?.modelsError }}
    </v-alert>
  </AppSectionCard>
</template>

<style scoped>
.help-setup-configuration-card__setup {
  display: grid;
  align-items: start;
  gap: 20px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  padding: 20px 24px;
}

.help-setup-configuration-card__field {
  display: grid;
  align-content: start;
  gap: 8px;
  min-width: 0;
}

.help-setup-configuration-card__label {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.help-setup-configuration-card__provider-select {
  min-width: 0;
}

.help-setup-configuration-card__provider-selection {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
}

.help-setup-configuration-card__provider-selection span:first-of-type {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.help-setup-configuration-card__provider-type {
  margin-left: 8px;
  color: currentColor;
  font-size: 0.75rem;
  opacity: 0.72;
  text-transform: capitalize;
}

.help-setup-configuration-card__provider-option-subtitle {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.82rem;
  text-transform: capitalize;
}

.help-setup-configuration-card__provider-meta {
  display: flex;
  align-items: center;
  min-width: 0;
  flex-wrap: wrap;
  gap: 8px;
}

.help-setup-configuration-card__hint {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.82rem;
}

@media (max-width: 820px) {
  .help-setup-configuration-card__setup {
    grid-template-columns: 1fr;
  }
}
</style>
