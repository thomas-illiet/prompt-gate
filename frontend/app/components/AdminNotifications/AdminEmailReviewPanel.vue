<script setup lang="ts">
import type {
  NotificationAudienceResult,
  NotificationPreview,
} from '~/types/notifications'

const props = defineProps<{
  audience: NotificationAudienceResult
  canSend: boolean
  preview: NotificationPreview
  saving: boolean
  subject: string
}>()

const emit = defineEmits<{
  edit: []
  send: []
}>()

const sendLabel = computed(() => {
  const count = props.audience.valid
  return `Queue ${count} email${count === 1 ? '' : 's'}`
})
</script>

<template>
  <AppSectionCard
    class="admin-email-review"
    icon="mdi-email-check-outline"
    title="Preview & confirm"
    subtitle="Review the final email and recipients before queuing delivery."
  >
    <div class="admin-email-review__body">
      <div class="admin-email-review__summary">
        <div class="admin-email-review__summary-item">
          <v-icon icon="mdi-account-multiple-check-outline" />
          <div>
            <span>Recipients</span>
            <strong>
              {{ props.audience.valid }} deliverable recipient{{
                props.audience.valid === 1 ? '' : 's'
              }}
            </strong>
          </div>
        </div>
        <div class="admin-email-review__summary-item">
          <v-icon icon="mdi-format-title" />
          <div>
            <span>Subject</span>
            <strong>{{ props.subject }}</strong>
          </div>
        </div>
      </div>

      <div class="admin-email-review__preview">
        <div class="admin-email-review__preview-heading">
          <div>
            <strong>Email preview</strong>
            <span>Sanitized HTML seen by recipients</span>
          </div>
        </div>

        <iframe
          class="admin-email-review__preview-content markdown-body"
          :srcdoc="props.preview.htmlBody"
          title="Email client preview"
          sandbox=""
        />
      </div>

      <div class="admin-email-review__actions">
        <v-btn
          prepend-icon="mdi-arrow-left"
          size="large"
          type="button"
          variant="outlined"
          @click="emit('edit')"
        >
          Back to edit
        </v-btn>
        <v-btn
          color="primary"
          :disabled="!props.canSend"
          :loading="props.saving"
          prepend-icon="mdi-send-outline"
          size="large"
          type="button"
          @click="emit('send')"
        >
          Confirm and {{ sendLabel.toLowerCase() }}
        </v-btn>
      </div>
      <p class="admin-email-review__delivery-note">
        Emails are queued and delivered asynchronously. Progress appears in
        Email delivery.
      </p>
    </div>
  </AppSectionCard>
</template>

<style scoped>
.admin-email-review {
  max-width: 1040px;
  width: 100%;
  margin-inline: auto;
}

.admin-email-review__body,
.admin-email-review__preview,
.admin-email-review__preview-heading > div {
  display: grid;
  gap: 12px;
}

.admin-email-review__body {
  gap: 20px;
  padding: 24px;
}

.admin-email-review__summary-item span,
.admin-email-review__preview-heading span,
.admin-email-review__delivery-note {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.8rem;
}

.admin-email-review__summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.admin-email-review__summary-item,
.admin-email-review__preview-heading {
  display: flex;
  align-items: center;
  gap: 12px;
}

.admin-email-review__summary-item {
  min-width: 0;
  padding: 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.44);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.42);
}

.admin-email-review__summary-item > div {
  display: grid;
  min-width: 0;
}

.admin-email-review__summary-item strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-email-review__preview {
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.48);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.4);
}

.admin-email-review__preview-heading {
  justify-content: space-between;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.4);
}

.admin-email-review__preview-content {
  display: block;
  width: 100%;
  min-height: 760px;
  border: 0;
  background: white;
}

.admin-email-review__delivery-note {
  margin: 0;
}

.admin-email-review__actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.admin-email-review__delivery-note {
  text-align: center;
}

@media (max-width: 600px) {
  .admin-email-review__summary {
    grid-template-columns: 1fr;
  }

  .admin-email-review__actions {
    align-items: stretch;
    flex-direction: column-reverse;
  }
}
</style>
