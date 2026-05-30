<script setup lang="ts">
import type { CreatedTokenResponse } from '~/types/service-accounts'

const props = defineProps<{
  createdToken: CreatedTokenResponse | null
}>()

const isOpen = defineModel<boolean>({ default: false })
</script>

<template>
  <AppCreatedTokenDialog v-model="isOpen" :created-token="props.createdToken" />
</template>
