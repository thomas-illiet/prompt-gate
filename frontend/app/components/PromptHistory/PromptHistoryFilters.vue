<script setup lang="ts">
import type { AdminUser } from '~/types/users'

const props = withDefaults(
  defineProps<{
    loadingUsers?: boolean
    search: string
    showUser?: boolean
    subtitle?: string
    title?: string
    userId?: string
    users?: AdminUser[]
  }>(),
  {
    loadingUsers: false,
    showUser: false,
    subtitle: 'Find prompts by text across your recorded proxy history.',
    title: 'Search prompts',
    userId: '',
    users: () => [],
  },
)

const emit = defineEmits<{
  'update:search': [value: string]
  'update:user-id': [value: string | null]
  'update:user-search': [value: string]
}>()

const userOptions = computed(() =>
  props.users.map((user) => ({
    email: user.email,
    title:
      user.name || user.preferredUsername || user.email || 'Unnamed account',
    value: user.id,
  })),
)
</script>

<template>
  <AppFilterCard :title="props.title" :subtitle="props.subtitle">
    <div class="prompt-history-filters__grid">
      <v-text-field
        :model-value="props.search"
        label="Search prompts"
        placeholder="Prompt text"
        prepend-inner-icon="mdi-magnify"
        variant="outlined"
        density="compact"
        flat
        hide-details
        clearable
        @update:model-value="emit('update:search', String($event ?? ''))"
      />

      <v-autocomplete
        v-if="props.showUser"
        :model-value="props.userId"
        :items="userOptions"
        :loading="props.loadingUsers"
        item-title="title"
        item-value="value"
        label="User"
        placeholder="All users"
        prepend-inner-icon="mdi-account-search-outline"
        variant="outlined"
        density="compact"
        flat
        hide-details
        clearable
        @update:model-value="
          emit('update:user-id', ($event as string | null) ?? null)
        "
        @update:search="emit('update:user-search', String($event ?? ''))"
      >
        <template #item="{ props: itemProps, item }">
          <v-list-item
            v-bind="itemProps"
            :subtitle="item.email"
            :title="item.title"
          />
        </template>
      </v-autocomplete>
    </div>
  </AppFilterCard>
</template>

<style scoped>
.prompt-history-filters__grid {
  display: grid;
  gap: 16px;
}

@media (min-width: 960px) {
  .prompt-history-filters__grid {
    grid-template-columns: minmax(0, 1.5fr) minmax(220px, 0.75fr);
    align-items: center;
  }
}
</style>
