<script setup lang="ts">
import QuotaExhaustedDialog from '~/components/QuotaExhaustion/QuotaExhaustedDialog.vue'
import { hasRequiredRole } from '~/utils/auth'

const authStore = useAuthStore()
const quotaExhaustionDialog = useQuotaExhaustionDialog()
const canViewMonitoring = computed(() =>
  hasRequiredRole(authStore.user, ['user', 'admin']),
)
</script>

<template>
  <v-app>
    <AppDrawer />
    <AppBar />
    <v-main class="default-layout-main">
      <AppMonitoringBanner v-if="canViewMonitoring" />
      <slot />
    </v-main>
    <AppFooter />
    <QuotaExhaustedDialog
      v-model="quotaExhaustionDialog.isOpen.value"
      :can-reset="quotaExhaustionDialog.canReset.value"
      :reset-available="quotaExhaustionDialog.resetAvailable.value"
      :reset-error="quotaExhaustionDialog.resetError.value"
      :reset-loading="quotaExhaustionDialog.resetLoading.value"
      :reset-maximum="quotaExhaustionDialog.resetMaximum.value"
      :reset-pending="quotaExhaustionDialog.resetPending.value"
      :reset-using="quotaExhaustionDialog.resetUsing.value"
      :windows="quotaExhaustionDialog.windows.value"
      @reset="quotaExhaustionDialog.useReset"
    />
  </v-app>
</template>

<style scoped>
.default-layout-main {
  padding-top: 0;
  padding-bottom: 0;
  margin-top: 64px;
  margin-bottom: 32px;
  min-height: calc(100dvh - 64px - 32px);
  transition-property: padding;
}
</style>
