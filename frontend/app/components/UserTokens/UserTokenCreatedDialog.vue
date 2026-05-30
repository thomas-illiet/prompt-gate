<script setup lang="ts">
import type { CreatedUserToken } from '~/types/user-service'

const props = defineProps<{
  createdToken: CreatedUserToken | null
}>()

const isOpen = defineModel<boolean>({ default: false })
</script>

<template>
  <AppCreatedTokenDialog v-model="isOpen" :created-token="props.createdToken" />
</template>
