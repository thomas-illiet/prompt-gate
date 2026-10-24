<script setup lang="ts">
const props = defineProps<{
  canSend: boolean
  error: string
  maxPromptLength: number
  running: boolean
}>()

const emit = defineEmits<{
  send: []
  stop: []
}>()

const draft = defineModel<string>({ required: true })
const input = useTemplateRef<{ focus: () => void }>('input')

function handleKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.isComposing || props.running) {
    return
  }
  event.preventDefault()
  if (props.canSend) emit('send')
}

function focus() {
  input.value?.focus()
}

defineExpose({ focus })
</script>

<template>
  <section class="model-lab-chat-composer" aria-label="Message composer">
    <v-text-field
      ref="input"
      v-model="draft"
      class="model-lab-chat-composer__input"
      enterkeyhint="send"
      hide-details="auto"
      label="Message"
      :disabled="props.running"
      :error-messages="props.error ? [props.error] : []"
      :maxlength="props.maxPromptLength"
      placeholder="Message the selected model…"
      type="text"
      variant="outlined"
      @keydown="handleKeydown"
    />

    <v-btn
      v-if="props.running"
      aria-label="Stop generating"
      color="warning"
      height="56"
      prepend-icon="mdi-stop-circle-outline"
      rounded="xl"
      variant="tonal"
      @click="emit('stop')"
    >
      Stop
    </v-btn>
    <v-btn
      v-else
      aria-label="Send message"
      color="primary"
      height="56"
      prepend-icon="mdi-send"
      rounded="xl"
      :disabled="!props.canSend"
      @click="emit('send')"
    >
      Send
    </v-btn>
  </section>
</template>

<style scoped>
.model-lab-chat-composer {
  display: flex;
  align-items: flex-start;
  flex: 0 0 auto;
  gap: 12px;
  padding: 14px 20px calc(14px + env(safe-area-inset-bottom));
  border-top: 1px solid rgba(var(--app-shell-border), 0.52);
  background: rgba(var(--app-shell-surface-strong), 0.96);
  backdrop-filter: blur(14px);
}

.model-lab-chat-composer__input {
  min-width: 0;
  flex: 1 1 auto;
}

@media (max-width: 520px) {
  .model-lab-chat-composer {
    gap: 8px;
    padding-inline: 14px;
  }
}
</style>
