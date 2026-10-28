<script setup lang="ts">
import type { AccessGroup } from '~/types/groups'
import type {
  NotificationAudienceMode,
  NotificationAudienceResult,
} from '~/types/notifications'
import type { AdminUser } from '~/types/users'

const props = defineProps<{
  audience: NotificationAudienceResult | null
  canResolve: boolean
  groups: AccessGroup[]
  loadingOptions: boolean
  resolving: boolean
  users: AdminUser[]
}>()

const emit = defineEmits<{ resolve: [] }>()
const mode = defineModel<NotificationAudienceMode>('mode', { required: true })
const externalAddress = defineModel<string>('externalAddress', {
  required: true,
})
const userIds = defineModel<string[]>('userIds', { required: true })
const groupIds = defineModel<string[]>('groupIds', { required: true })

const modes: Array<{
  icon: string
  label: string
  value: NotificationAudienceMode
}> = [
  { icon: 'mdi-account-multiple-outline', label: 'Users', value: 'users' },
  { icon: 'mdi-account-group-outline', label: 'Groups', value: 'groups' },
  { icon: 'mdi-account-network-outline', label: 'Everyone', value: 'all' },
  { icon: 'mdi-email-outline', label: 'External', value: 'external' },
]

const userItems = computed(() =>
  props.users.map((user) => ({
    title: user.name || user.preferredUsername || user.email,
    value: user.id,
    props: {
      subtitle: user.email,
    },
  })),
)
const groupItems = computed(() =>
  props.groups.map((group) => ({
    title: group.displayName || group.name,
    value: group.id,
    props: {
      subtitle: group.name,
    },
  })),
)
</script>

<template>
  <section
    class="admin-email-audience"
    aria-labelledby="email-audience-heading"
  >
    <div class="admin-email-audience__heading">
      <v-avatar color="primary" size="32" variant="tonal">1</v-avatar>
      <div>
        <h2 id="email-audience-heading">Choose recipients</h2>
        <p>Select a target, then verify the exact deliverable audience.</p>
      </div>
    </div>

    <v-btn-toggle
      v-model="mode"
      class="admin-email-audience__modes"
      color="primary"
      mandatory
      variant="outlined"
    >
      <v-btn
        v-for="item in modes"
        :key="item.value"
        :value="item.value"
        :prepend-icon="item.icon"
      >
        {{ item.label }}
      </v-btn>
    </v-btn-toggle>

    <v-autocomplete
      v-if="mode === 'users'"
      v-model="userIds"
      chips
      closable-chips
      :items="userItems"
      label="Recipients"
      :loading="props.loadingOptions"
      multiple
      placeholder="Search by name or email"
      variant="outlined"
    />
    <v-autocomplete
      v-else-if="mode === 'groups'"
      v-model="groupIds"
      chips
      closable-chips
      :items="groupItems"
      label="Access groups"
      :loading="props.loadingOptions"
      multiple
      placeholder="Select one or more groups"
      variant="outlined"
    />
    <v-text-field
      v-else-if="mode === 'external'"
      v-model="externalAddress"
      autocomplete="email"
      label="External email address"
      placeholder="name@example.com"
      type="email"
      variant="outlined"
    />
    <p v-else class="admin-email-audience__hint">
      <v-icon icon="mdi-information-outline" size="18" />
      Every human account with a valid email address will be included, including
      currently suspended accounts.
    </p>

    <div class="admin-email-audience__verification">
      <div class="admin-email-audience__result" aria-live="polite">
        <template v-if="props.audience">
          <v-icon color="success" icon="mdi-check-circle" />
          <span>
            <strong>{{ props.audience.valid }}</strong> deliverable recipient{{
              props.audience.valid === 1 ? '' : 's'
            }}
            <template v-if="props.audience.ignored">
              · {{ props.audience.ignored }} ignored
            </template>
          </span>
        </template>
        <template v-else>
          <v-icon icon="mdi-account-search-outline" />
          <span>Audience not checked yet</span>
        </template>
      </div>
      <v-btn
        color="primary"
        :disabled="!props.canResolve"
        :loading="props.resolving"
        prepend-icon="mdi-account-check-outline"
        variant="tonal"
        @click="emit('resolve')"
      >
        {{ props.audience ? 'Check again' : 'Check recipients' }}
      </v-btn>
    </div>
  </section>
</template>

<style scoped>
.admin-email-audience {
  display: grid;
  gap: 20px;
}

.admin-email-audience__heading,
.admin-email-audience__verification,
.admin-email-audience__result {
  display: flex;
  align-items: center;
  gap: 12px;
}

.admin-email-audience__heading h2,
.admin-email-audience__heading p {
  margin: 0;
}

.admin-email-audience__heading h2 {
  font-size: 1rem;
  font-weight: 800;
}

.admin-email-audience__heading p,
.admin-email-audience__result,
.admin-email-audience__hint {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.875rem;
}

.admin-email-audience__hint {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
}

.admin-email-audience__modes {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  width: 100%;
  height: auto;
}

.admin-email-audience__modes :deep(.v-btn) {
  min-width: 0;
  height: 48px;
  padding-inline: 10px;
}

.admin-email-audience__verification {
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.44);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.42);
}

@media (max-width: 720px) {
  .admin-email-audience__modes {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .admin-email-audience__verification {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
