<script setup lang="ts">
import type { SetupGuidePayload } from '~/types/setup-guides'
import {
  renderSetupGuideTemplate,
  validateSetupGuideTemplate,
} from '~/utils/setup-guide-template'

const props = defineProps<{ guide: SetupGuidePayload }>()
const validationError = computed(() =>
  validateSetupGuideTemplate(props.guide.template),
)
const preview = computed(() =>
  validationError.value
    ? ''
    : renderSetupGuideTemplate(props.guide.template, {
        token: '<PROMPTGATE_TOKEN>',
        baseUrl: 'https://promptgate.example.com/v1',
        openaiBaseUrl: 'https://promptgate.example.com/v1',
        anthropicBaseUrl: 'https://promptgate.example.com/anthropic',
        model: 'gpt-example',
        models: ['gpt-example', 'gpt-fast'],
        providerName: 'example',
        providerDisplayName: 'Example provider',
      }),
)
</script>

<template>
  <v-alert v-if="validationError" type="error" variant="tonal">{{
    validationError
  }}</v-alert>
  <div v-else class="admin-setup-preview">
    <div class="text-overline mb-2">Live preview</div>
    <pre>{{ preview }}</pre>
  </div>
</template>

<style scoped>
.admin-setup-preview pre {
  max-height: 360px;
  overflow: auto;
  padding: 16px;
  border-radius: 10px;
  background: rgb(var(--v-theme-surface-variant));
  white-space: pre-wrap;
}
</style>
