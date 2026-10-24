<script setup lang="ts">
const route = useRoute()
const title = computed(() => {
  return route.meta?.title || route.matched[0]?.meta?.title || ''
})
useHead({
  title,
  titleTemplate: (t) => (t ? `${t} | PromptGate` : 'PromptGate'),
  htmlAttrs: { lang: 'en' },
  link: [{ rel: 'icon', href: '/favicon.ico' }],
})
useSeoMeta({
  viewport: 'width=device-width, initial-scale=1',
  description:
    'PromptGate provides secure, governed access to every AI model your organization is allowed to use.',
})
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
