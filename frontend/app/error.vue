<script setup lang="ts">
import type { NuxtError } from '#app'

import AppErrorState from '~/components/App/AppErrorState.vue'
import { getErrorPageContent } from '~/utils/error-page'

const props = defineProps<{
  error: NuxtError
}>()

const statusCode = computed(() => Number(props.error.statusCode || 500))
const content = computed(() => getErrorPageContent(statusCode.value))

const retryRedirect = computed(() => {
  if (!import.meta.client) return '/'

  return `${window.location.pathname}${window.location.search}${window.location.hash}`
})

const primaryRedirect = computed(() => {
  if (content.value.code === 401) {
    return `/login?redirect=${encodeURIComponent(retryRedirect.value)}`
  }

  return content.value.primaryRedirect
})

async function runPrimaryAction() {
  await clearError({
    redirect: content.value.retry ? retryRedirect.value : primaryRedirect.value,
  })
}

async function returnToDashboard() {
  await clearError({ redirect: '/dashboard' })
}
</script>

<template>
  <NuxtLayout name="public">
    <v-container class="global-error-page">
      <AppErrorState
        :code="content.code"
        :eyebrow="content.eyebrow"
        :text="content.text"
        :title="content.title"
        :variant="content.variant"
      >
        <template #actions>
          <v-btn
            color="primary"
            variant="flat"
            rounded="lg"
            @click="runPrimaryAction"
          >
            {{ content.primaryLabel }}
          </v-btn>
          <v-btn
            v-if="content.retry"
            variant="tonal"
            rounded="lg"
            @click="returnToDashboard"
          >
            Back to dashboard
          </v-btn>
        </template>
      </AppErrorState>
    </v-container>
  </NuxtLayout>
</template>

<style scoped>
.global-error-page {
  display: grid;
  min-height: 100dvh;
  place-items: center;
  padding-block: 32px;
}
</style>
