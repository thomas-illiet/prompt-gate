<script setup lang="ts">
import type { DashboardMetric } from '~/types/user-service'

const props = withDefaults(
  defineProps<{
    ariaLabel?: string
  }>(),
  {
    ariaLabel: 'Dashboard metric',
  },
)

const metric = defineModel<DashboardMetric>({ required: true })

const options: { icon: string; label: string; value: DashboardMetric }[] = [
  { icon: 'mdi-api', label: 'Requests', value: 'requests' },
  { icon: 'mdi-counter', label: 'Tokens', value: 'tokens' },
  { icon: 'mdi-currency-usd', label: 'Cost', value: 'cost' },
]
</script>

<template>
  <v-btn-toggle
    v-model="metric"
    :aria-label="props.ariaLabel"
    color="primary"
    density="compact"
    mandatory
    variant="outlined"
  >
    <v-btn
      v-for="option in options"
      :key="option.value"
      :aria-label="`Show ${option.label.toLowerCase()}`"
      :value="option.value"
      size="small"
    >
      <v-icon :icon="option.icon" start />
      {{ option.label }}
    </v-btn>
  </v-btn-toggle>
</template>
