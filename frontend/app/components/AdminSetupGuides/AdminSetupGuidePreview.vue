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
  <div v-else class="admin-setup-preview app-markdown">
    <div class="admin-setup-preview__header">
      <v-icon icon="mdi-code-tags" size="18" />
      <span>Rendered output</span>
    </div>
    <pre><code>{{ preview }}</code></pre>
  </div>
</template>

<style scoped>
.admin-setup-preview__header {
  display: flex;
  align-items: center;
  gap: 8px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.admin-setup-preview pre {
  max-height: 420px;
  margin-block: 12px 0;
  overflow-x: hidden;
}

.admin-setup-preview pre code {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}
</style>
