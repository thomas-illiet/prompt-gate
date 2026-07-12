<script setup lang="ts">
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    closeLabel?: string
    closable?: boolean
    contentClass?: string
    describedBy?: string
    icon?: string
    iconColor?: string
    loading?: boolean
    maxWidth?: number | string
    persistent?: boolean
    subtitle?: string
    title: string
  }>(),
  {
    closeLabel: 'Close dialog',
    closable: true,
    contentClass: '',
    describedBy: '',
    icon: '',
    iconColor: 'primary',
    loading: false,
    maxWidth: 640,
    persistent: false,
    subtitle: '',
  },
)

const isOpen = defineModel<boolean>({ default: false })
const generatedId = useId()
const titleId = `app-dialog-title-${generatedId}`
const subtitleId = `app-dialog-subtitle-${generatedId}`
const accessibleDescription = computed(() =>
  props.describedBy || (props.subtitle ? subtitleId : undefined),
)

function close() {
  if (props.loading || props.persistent) {
    return
  }

  isOpen.value = false
}
</script>

<template>
  <v-dialog
    v-model="isOpen"
    :max-width="props.maxWidth"
    :persistent="props.persistent || props.loading"
    :aria-labelledby="titleId"
    :aria-describedby="accessibleDescription"
    scrollable
  >
    <v-card rounded="lg" class="app-dialog-card app-surface-gradient">
      <v-card-item class="app-dialog-card__header">
        <template v-if="props.icon" #prepend>
          <v-avatar :color="props.iconColor" variant="tonal" size="44">
            <v-icon :icon="props.icon" />
          </v-avatar>
        </template>

        <v-card-title :id="titleId" class="app-dialog-card__title text-h6">
          {{ props.title }}
        </v-card-title>
        <v-card-subtitle
          v-if="props.subtitle"
          :id="subtitleId"
          class="app-dialog-card__subtitle"
        >
          {{ props.subtitle }}
        </v-card-subtitle>

        <template v-if="props.closable" #append>
          <v-btn
            :aria-label="props.closeLabel"
            :disabled="props.loading || props.persistent"
            icon="mdi-close"
            size="small"
            variant="text"
            @click="close"
          />
        </template>
      </v-card-item>

      <v-card-text
        v-if="$slots.default"
        :class="['app-dialog-card__content', props.contentClass]"
      >
        <slot />
      </v-card-text>

      <v-card-actions v-if="$slots.actions" class="app-dialog-card__actions">
        <slot name="actions" />
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.app-dialog-card {
  max-height: min(90dvh, 960px);
  overflow: hidden;
}

.app-dialog-card__header {
  flex: 0 0 auto;
  padding: 24px 24px 10px;
}

.app-dialog-card__title,
.app-dialog-card__subtitle {
  overflow-wrap: anywhere;
  white-space: normal;
}

.app-dialog-card__subtitle {
  margin-top: 2px;
  line-height: 1.45;
}

.app-dialog-card__content {
  flex: 1 1 auto;
  padding: 14px 24px 10px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.app-dialog-card__actions {
  flex: 0 0 auto;
  gap: 10px;
  justify-content: flex-end;
  padding: 12px 24px 24px;
}

@media (max-width: 600px) {
  .app-dialog-card {
    width: calc(100vw - 24px);
    max-height: calc(100dvh - 24px);
  }

  .app-dialog-card__header {
    padding: 18px 16px 8px;
  }

  .app-dialog-card__content {
    padding: 12px 16px 8px;
  }

  .app-dialog-card__actions {
    display: flex;
    flex-direction: column-reverse;
    align-items: stretch;
    padding: 10px 16px 16px;
  }

  .app-dialog-card__actions :deep(.v-btn) {
    width: 100%;
  }
}
</style>
