<script setup lang="ts">
definePageMeta({
  icon: 'mdi-frequently-asked-questions',
  title: 'FAQ',
  drawerIndex: 5,
  requiredRoles: ['user', 'manager', 'admin'],
})

const faq = useFAQ()
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12"><AppPageHero icon="mdi-frequently-asked-questions" kicker="Documentation" title="Frequently asked questions" copy="Browse answers and guidance maintained by your administrators." stat-label="Published answers" :stat-value="String(faq.entries.value.length)" /></v-col>
      <v-col v-if="faq.error.value" cols="12"><v-alert type="warning" variant="tonal" rounded="lg" closable>{{ faq.error.value }}<template #append><v-btn variant="text" @click="faq.load">Retry</v-btn></template></v-alert></v-col>
      <v-col cols="12"><FAQList :entries="faq.entries.value" :loading="faq.loading.value" /></v-col>
    </v-row>
  </v-container>
</template>
