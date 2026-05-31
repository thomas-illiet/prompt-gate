<script setup lang="ts">
interface QuickAction {
  icon: string
  title: string
  to: string
}

const props = defineProps<{
  actions: QuickAction[]
}>()

const emit = defineEmits<{
  logout: []
}>()
</script>

<template>
  <ProfileInfoCard
    icon="mdi-lightning-bolt-outline"
    title="Actions"
    subtitle="Common account tools"
  >
    <v-list density="compact" class="profile-quick-actions">
      <v-list-item
        v-for="action in props.actions"
        :key="action.to"
        :to="action.to"
        rounded="lg"
        class="profile-quick-actions__item"
      >
        <template #prepend>
          <v-icon :icon="action.icon" />
        </template>
        <v-list-item-title>{{ action.title }}</v-list-item-title>
        <template #append>
          <v-icon icon="mdi-chevron-right" size="20" />
        </template>
      </v-list-item>

      <v-divider class="my-2" />

      <v-list-item
        rounded="lg"
        prepend-icon="mdi-logout"
        title="Logout"
        class="profile-quick-actions__logout"
        @click="emit('logout')"
      />
    </v-list>
  </ProfileInfoCard>
</template>

<style scoped>
.profile-quick-actions {
  display: grid;
  gap: 4px;
  padding: 8px;
}

.profile-quick-actions__item,
.profile-quick-actions__logout {
  min-height: 44px;
}

.profile-quick-actions__item {
  color: rgb(var(--app-shell-text-primary));
}

.profile-quick-actions__item:hover {
  background: rgba(var(--v-theme-primary), 0.07);
}

.profile-quick-actions__logout {
  color: rgb(var(--v-theme-error));
}
</style>
