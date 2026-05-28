<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    icon?: string
    iconColor?: string
    loading?: boolean
    maxWidth?: number | string
    persistent?: boolean
    subtitle?: string
    title: string
  }>(),
  {
    icon: '',
    iconColor: 'primary',
    loading: false,
    maxWidth: 640,
    persistent: false,
    subtitle: '',
  },
)

const isOpen = defineModel<boolean>({ default: false })
</script>

<template>
  <v-dialog
    v-model="isOpen"
    :max-width="props.maxWidth"
    :persistent="props.persistent || props.loading"
  >
    <v-card rounded="lg" class="app-dialog-card app-surface-gradient">
      <v-card-item class="px-6 pt-6 pb-2">
        <template v-if="props.icon" #prepend>
          <v-avatar :color="props.iconColor" variant="tonal" size="44">
            <v-icon :icon="props.icon" />
          </v-avatar>
        </template>

        <v-card-title class="text-h6">{{ props.title }}</v-card-title>
        <v-card-subtitle v-if="props.subtitle">
          {{ props.subtitle }}
        </v-card-subtitle>
      </v-card-item>

      <v-card-text v-if="$slots.default" class="px-6 pt-4 pb-2">
        <slot />
      </v-card-text>

      <v-card-actions v-if="$slots.actions" class="px-6 pb-6">
        <slot name="actions" />
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
