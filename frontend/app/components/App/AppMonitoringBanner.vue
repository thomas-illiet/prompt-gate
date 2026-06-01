<script setup lang="ts">
const monitoring = useMonitoringStatus()

const degradedServices = computed(() => monitoring.services.value)
const showBanner = computed(() => degradedServices.value.length > 0)
const serviceNames = computed(() =>
  degradedServices.value.map((service) => service.displayName || service.name),
)
const servicesLabel = computed(() => {
  const visible = serviceNames.value.slice(0, 3)
  const remaining = serviceNames.value.length - visible.length
  if (remaining <= 0) {
    return visible.join(', ')
  }

  return `${visible.join(', ')} +${remaining}`
})
const message = computed(() =>
  serviceNames.value.length === 1
    ? `Service perturbe: ${servicesLabel.value}`
    : `Services perturbes: ${servicesLabel.value}`,
)
</script>

<template>
  <v-alert
    v-if="showBanner"
    type="warning"
    variant="flat"
    density="compact"
    icon="mdi-alert-circle-outline"
    class="app-monitoring-banner"
  >
    {{ message }}
  </v-alert>
</template>

<style scoped>
.app-monitoring-banner {
  position: sticky;
  top: 0;
  z-index: 2;
  border-radius: 0;
}
</style>
