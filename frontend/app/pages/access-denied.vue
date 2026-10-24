<script setup lang="ts">
import { storeToRefs } from 'pinia'

import AppErrorState from '~/components/App/AppErrorState.vue'

definePageMeta({
  allowBlocked: true,
  layout: 'public',
  title: 'Access denied',
})

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const message = computed(() => {
  if (!user.value?.isActive) {
    return 'Your account is inactive. An administrator needs to reactivate it before you can continue.'
  }
  if (user.value?.role === 'none') {
    return 'Your platform access has been removed. Contact an administrator if you believe this is a mistake.'
  }
  return 'You are signed in, but you do not have permission to open this section.'
})

const canReturnToDashboard = computed(
  () => Boolean(user.value?.isActive) && user.value?.role !== 'none',
)

// logout signs the blocked user out through the auth store.
async function logout() {
  await authStore.logout('/login')
}
</script>

<template>
  <v-container class="access-denied-page">
    <AppErrorState
      :code="403"
      eyebrow="Restricted workspace"
      title="Access denied"
      :text="message"
      variant="forbidden"
    >
      <template #actions>
        <v-btn
          v-if="canReturnToDashboard"
          to="/dashboard"
          color="primary"
          variant="flat"
          rounded="lg"
          prepend-icon="mdi-monitor-dashboard"
        >
          Back to dashboard
        </v-btn>
        <v-btn
          variant="tonal"
          rounded="lg"
          prepend-icon="mdi-logout"
          @click="logout"
        >
          Logout
        </v-btn>
      </template>
    </AppErrorState>
  </v-container>
</template>

<style scoped>
.access-denied-page {
  display: grid;
  min-height: 100dvh;
  place-items: center;
  padding-block: 32px;
}
</style>
