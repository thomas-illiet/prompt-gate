<script setup lang="ts">
import ModelLabChatPanel from '~/components/ModelLab/ModelLabChatPanel.vue'
import { copyTextToClipboard } from '~/utils/clipboard'
import { flattenModelLabCatalog } from '~/utils/model-lab'

definePageMeta({
  icon: 'mdi-flask-outline',
  path: '/models',
  title: 'Model Lab',
  drawerIndex: 3,
  requiredRoles: ['user', 'admin'],
})

const catalog = useModelLabCatalog()
const targets = computed(() =>
  flattenModelLabCatalog(catalog.catalog.value?.providers ?? []),
)
const limits = computed(() => catalog.catalog.value?.limits)
const chat = useModelLabChat(targets, limits)

async function copyOutput(text: string) {
  if (await copyTextToClipboard(text)) {
    Notify.success('Response copied to the clipboard.')
    return
  }
  Notify.warning('The response could not be copied.')
}

function sendMessage() {
  void chat.send()
}

function retryLane(turnId: string, laneId: string) {
  void chat.retryLane(turnId, laneId)
}
</script>

<template>
  <v-container fluid class="app-page model-lab-page">
    <ModelLabChatPanel
      :draft="chat.draft.value"
      :draft-error="chat.draftError.value"
      :error="chat.error.value"
      :max-allowed-output-tokens="chat.maxAllowedOutputTokens.value"
      :max-output-tokens="chat.maxOutputTokens.value"
      :max-prompt-length="chat.maxPromptLength.value"
      :max-selected-targets="chat.maxSelectedTargets.value"
      :models-error="catalog.error.value"
      :models-loading="catalog.loading.value"
      :running="chat.running.value"
      :selected-targets="chat.selectedTargets.value"
      :targets="targets"
      :temperature="chat.temperature.value"
      :turns="chat.turns.value"
      @cancel="chat.cancelLane"
      @copy="copyOutput"
      @new-chat="chat.reset"
      @retry="retryLane"
      @retry-models="catalog.reload"
      @send="sendMessage"
      @stop="chat.stopAll"
      @update:draft="chat.setDraft"
      @update:max-output-tokens="chat.setMaxOutputTokens"
      @update:selected-targets="chat.setSelectedTargets"
      @update:temperature="chat.setTemperature"
    />
  </v-container>
</template>

<style scoped>
.model-lab-page {
  display: flex;
  height: calc(100dvh - 64px - 32px);
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
}
</style>
