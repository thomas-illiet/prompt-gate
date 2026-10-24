<script setup lang="ts">
const notificationStore = useNotificationStore()
const { notifications } = storeToRefs(notificationStore)
const notificationsShown = computed(() =>
  notifications.value.filter((notification) => notification.show).reverse(),
)
const showAll = shallowRef(false)
const panelVisible = computed(
  () => showAll.value || notificationsShown.value.length > 0,
)
const timeout = computed(() => (showAll.value ? -1 : 5000))
const panelId = `app-notifications-${useId()}`
const notificationLabel = computed(() => {
  const count = notifications.value.length
  if (count === 0) return 'Notifications'
  return count === 1 ? '1 notification' : `${count} notifications`
})
const toggleLabel = computed(() => {
  if (showAll.value) return 'Hide notifications'
  if (notifications.value.length === 0) return 'Show notifications'
  return `Show ${notificationLabel.value}`
})
// deleteNotification removes one notification from the store.
function deleteNotification(id: number) {
  notificationStore.delNotification(id)
}
// emptyNotifications clears every notification currently shown.
function emptyNotifications() {
  notificationStore.$reset()
}
// toggleAll switches between compact and expanded notification display.
function toggleAll() {
  showAll.value = !showAll.value
  notifications.value.forEach((m) => {
    m.show = showAll.value
  })
}
</script>

<template>
  <v-btn
    v-tooltip="{ text: toggleLabel, location: 'top' }"
    :aria-controls="panelId"
    :aria-expanded="showAll"
    :aria-label="toggleLabel"
    :title="toggleLabel"
    :icon="notifications.length ? 'mdi-bell-badge-outline' : 'mdi-bell-outline'"
    rounded="lg"
    data-test="notification-trigger"
    @click="toggleAll"
  />
  <ClientOnly>
    <teleport to="body">
      <v-card
        :id="panelId"
        :aria-label="notificationLabel"
        :aria-hidden="!panelVisible"
        elevation="0"
        rounded="lg"
        class="d-flex flex-column notification-card app-surface-gradient"
        :class="{ 'notification-card--open': showAll }"
        data-test="notification-panel"
        role="region"
      >
        <v-toolbar flat density="compact">
          <v-toolbar-title
            class="font-weight-light text-body-large"
            :text="notificationLabel"
          />
          <v-btn
            v-if="notifications.length"
            v-tooltip="{ text: 'Clear all notifications', location: 'top' }"
            aria-label="Clear all notifications"
            size="small"
            icon="mdi-bell-remove"
            data-test="notification-clear"
            @click="emptyNotifications"
          />
          <v-btn
            v-tooltip="{ text: 'Hide notifications', location: 'top' }"
            aria-label="Hide notifications"
            class="mr-0"
            size="small"
            icon="$expand"
            @click="toggleAll"
          />
        </v-toolbar>
        <AppEmptyState
          v-if="showAll && notificationsShown.length === 0"
          compact
          icon="mdi-bell-check-outline"
          title="Nothing on the wire"
          text="New control-room signals will land here."
        />
        <v-slide-y-reverse-transition
          v-else
          tag="div"
          class="d-flex flex-column notification-box"
          group
          hide-on-leave
        >
          <div
            v-for="notification in notificationsShown"
            :key="notification.id"
            class="notification-item-wrapper"
          >
            <AppNotificationItem
              v-model="notification.show"
              :notification="notification"
              :timeout="timeout"
              class="notification-item"
              @close="deleteNotification(notification.id)"
            />
          </div>
        </v-slide-y-reverse-transition>
      </v-card>
    </teleport>
  </ClientOnly>
</template>

<style scoped>
.notification-item {
  width: 100%;
  border: 0;
}
.notification-card {
  z-index: 1006;
  position: fixed;
  right: 12px;
  bottom: 48px;
  width: min(400px, calc(100vw - 24px));
  max-height: calc(100dvh - 72px);
  overflow: visible;
  visibility: hidden;
  &.notification-card--open {
    visibility: visible;
    overflow: hidden;
    max-height: calc(100dvh - 72px);
    .notification-box {
      overflow-y: auto;
      pointer-events: auto;
      .notification-item-wrapper {
        transition: none;
        .notification-item {
          margin-bottom: 0;
          border-radius: 0;
          border-top: 1px solid rgba(var(--app-shell-border), 0.42);
        }
      }
    }
  }
}
.notification-box {
  overflow-y: visible;
  visibility: visible;
  pointer-events: none;
  .notification-item {
    pointer-events: initial;
    user-select: initial;
    margin-bottom: 10px;
  }
}

@media (max-width: 600px) {
  .notification-card {
    right: 8px;
    bottom: 48px;
    width: calc(100vw - 16px);
  }
}
</style>
