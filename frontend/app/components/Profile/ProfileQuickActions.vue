<script setup lang="ts">
interface QuickAction {
  icon: string
  subtitle: string
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
    title="Quick actions"
    subtitle="Jump to common account tools"
  >
    <v-list density="comfortable" class="profile-list">
      <v-list-item
        v-for="action in props.actions"
        :key="action.to"
        :to="action.to"
        rounded="lg"
      >
        <template #prepend>
          <v-icon :icon="action.icon" />
        </template>
        <v-list-item-title>{{ action.title }}</v-list-item-title>
        <v-list-item-subtitle>
          {{ action.subtitle }}
        </v-list-item-subtitle>
        <template #append>
          <v-icon icon="mdi-chevron-right" size="20" />
        </template>
      </v-list-item>

      <v-divider class="my-2" />

      <v-list-item
        rounded="lg"
        prepend-icon="mdi-logout"
        title="Logout"
        subtitle="End current session"
        @click="emit('logout')"
      />
    </v-list>
  </ProfileInfoCard>
</template>

<style scoped>
.profile-list {
  padding: 8px;
}
</style>
