<script setup lang="ts">
definePageMeta({
  icon: 'mdi-frequently-asked-questions',
  title: 'FAQ',
  drawerIndex: 5,
  requiredRoles: ['limited', 'user', 'admin'],
})

const faq = useFAQ()
const publishedAnswers = computed(() =>
  faq.loading.value ? '…' : String(faq.entries.value.length),
)
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-frequently-asked-questions"
          kicker="Field manual"
          title="Frequently asked questions"
          copy="Answers without the scavenger hunt. Search the guidance published by your administrators."
          visual="book"
          stat-label="Published answers"
          :stat-value="publishedAnswers"
        />
      </v-col>

      <v-col v-if="faq.error.value" cols="12">
        <v-alert type="warning" variant="tonal" rounded="lg" closable>
          {{ faq.error.value }}
          <template #append>
            <v-btn
              data-test="faq-retry"
              prepend-icon="mdi-refresh"
              variant="text"
              @click="faq.load"
            >
              Retry
            </v-btn>
          </template>
        </v-alert>
      </v-col>

      <v-col cols="12">
        <FAQList :entries="faq.entries.value" :loading="faq.loading.value" />
      </v-col>
    </v-row>
  </v-container>
</template>
