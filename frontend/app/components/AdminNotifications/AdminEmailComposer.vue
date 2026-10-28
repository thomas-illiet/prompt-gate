<script setup lang="ts">
import AdminEmailAudiencePanel from '~/components/AdminNotifications/AdminEmailAudiencePanel.vue'
import AdminEmailReviewPanel from '~/components/AdminNotifications/AdminEmailReviewPanel.vue'
import type { AccessGroup } from '~/types/groups'
import type { AdminUser } from '~/types/users'
import type {
  NotificationAudienceMode,
  NotificationAudiencePayload,
  NotificationAudienceResult,
  NotificationPreview,
  SendNotificationPayload,
} from '~/types/notifications'

const props = defineProps<{
  audience: NotificationAudienceResult | null
  groups: AccessGroup[]
  loadingOptions: boolean
  preview: NotificationPreview | null
  previewing: boolean
  resolving: boolean
  saving: boolean
  users: AdminUser[]
}>()

const emit = defineEmits<{
  preview: [payload: Pick<SendNotificationPayload, 'subject' | 'markdown'>]
  resolve: [audience: NotificationAudiencePayload]
  send: [payload: SendNotificationPayload]
  'invalidate:audience': []
  'invalidate:preview': []
}>()

const mode = shallowRef<NotificationAudienceMode>('users')
const externalAddress = shallowRef('')
const userIds = shallowRef<string[]>([])
const groupIds = shallowRef<string[]>([])
const subject = shallowRef('')
const markdown = shallowRef('')
const step = shallowRef<'compose' | 'review'>('compose')
const awaitingPreview = shallowRef(false)
const audiencePayload = computed<NotificationAudiencePayload>(() => ({
  mode: mode.value,
  externalAddress: externalAddress.value,
  userIds: userIds.value,
  groupIds: groupIds.value,
}))

const canResolveAudience = computed(() => {
  if (mode.value === 'external') return Boolean(externalAddress.value.trim())
  if (mode.value === 'users') return userIds.value.length > 0
  if (mode.value === 'groups') return groupIds.value.length > 0
  return true
})
const hasSubject = computed(() => Boolean(subject.value.trim()))
const hasBody = computed(() => Boolean(markdown.value.trim()))
const canContinue = computed(
  () => hasSubject.value && hasBody.value && Boolean(props.audience?.valid),
)
const canSend = computed(
  () =>
    hasSubject.value &&
    hasBody.value &&
    Boolean(props.audience?.valid) &&
    Boolean(props.preview),
)

watch([mode, externalAddress, userIds, groupIds], () => {
  emit('invalidate:audience')
})

watch([subject, markdown], () => {
  emit('invalidate:preview')
})

watch(
  () => props.preview,
  (preview) => {
    if (!preview || !awaitingPreview.value) return
    awaitingPreview.value = false
    step.value = 'review'
  },
)

watch(
  () => props.previewing,
  (previewing) => {
    if (!previewing && awaitingPreview.value && !props.preview) {
      awaitingPreview.value = false
    }
  },
)

function continueToPreview() {
  if (!canContinue.value) return
  if (props.preview) {
    step.value = 'review'
    return
  }
  awaitingPreview.value = true
  emit('preview', {
    subject: subject.value.trim(),
    markdown: markdown.value,
  })
}

function editMessage() {
  step.value = 'compose'
}

function send() {
  if (!canSend.value) return
  emit('send', {
    subject: subject.value.trim(),
    markdown: markdown.value,
    audience: audiencePayload.value,
  })
}
</script>

<template>
  <div class="admin-email-composer">
    <nav
      class="admin-email-composer__steps"
      aria-label="Email creation progress"
    >
      <div
        class="admin-email-composer__step"
        :class="{ 'admin-email-composer__step--active': step === 'compose' }"
      >
        <v-avatar color="primary" size="32" variant="tonal">
          <v-icon icon="mdi-pencil-outline" size="18" />
        </v-avatar>
        <div>
          <strong>Compose</strong>
          <span>Recipients and message</span>
        </div>
      </div>
      <v-icon
        class="admin-email-composer__step-arrow"
        icon="mdi-chevron-right"
      />
      <div
        class="admin-email-composer__step"
        :class="{ 'admin-email-composer__step--active': step === 'review' }"
      >
        <v-avatar color="primary" size="32" variant="tonal">
          <v-icon icon="mdi-email-check-outline" size="18" />
        </v-avatar>
        <div>
          <strong>Preview & confirm</strong>
          <span>Final check before sending</span>
        </div>
      </div>
    </nav>

    <div v-if="step === 'compose'" class="admin-email-composer__compose">
      <AppSectionCard
        icon="mdi-email-edit-outline"
        title="Compose message"
        subtitle="Choose the recipients and write the message. You will review the final email before it is sent."
      >
        <div class="admin-email-composer__body">
          <AdminEmailAudiencePanel
            v-model:external-address="externalAddress"
            v-model:group-ids="groupIds"
            v-model:mode="mode"
            v-model:user-ids="userIds"
            :audience="props.audience"
            :can-resolve="canResolveAudience"
            :groups="props.groups"
            :loading-options="props.loadingOptions"
            :resolving="props.resolving"
            :users="props.users"
            @resolve="emit('resolve', audiencePayload)"
          />

          <v-divider />

          <section
            class="admin-email-composer__message"
            aria-labelledby="email-message-heading"
          >
            <div class="admin-email-composer__section-heading">
              <v-avatar color="primary" size="32" variant="tonal">2</v-avatar>
              <div>
                <h2 id="email-message-heading">Write the message</h2>
                <p>Use Markdown for headings, lists, links, and emphasis.</p>
              </div>
            </div>

            <v-text-field
              v-model="subject"
              autocomplete="off"
              counter="255"
              label="Subject"
              maxlength="255"
              placeholder="What should recipients know?"
              variant="outlined"
            />
            <v-textarea
              v-model="markdown"
              auto-grow
              hint="The final HTML is sanitized before it is queued."
              label="Message"
              persistent-hint
              placeholder="Write a clear, concise message…"
              rows="10"
              variant="outlined"
            />
          </section>

          <div class="admin-email-composer__continue">
            <div>
              <strong>Ready for the final check?</strong>
              <span>
                The next step shows the sanitized email exactly as recipients
                will see it.
              </span>
            </div>
            <v-btn
              color="primary"
              :disabled="!canContinue"
              :loading="props.previewing && awaitingPreview"
              append-icon="mdi-arrow-right"
              size="large"
              type="button"
              @click="continueToPreview"
            >
              Continue to preview
            </v-btn>
          </div>
        </div>
      </AppSectionCard>
    </div>

    <AdminEmailReviewPanel
      v-else-if="props.audience && props.preview"
      :audience="props.audience"
      :can-send="canSend"
      :preview="props.preview"
      :saving="props.saving"
      :subject="subject.trim()"
      @edit="editMessage"
      @send="send"
    />
  </div>
</template>

<style scoped>
.admin-email-composer {
  display: grid;
  gap: 24px;
}

.admin-email-composer__steps {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 16px 20px;
  border: 1px solid rgba(var(--app-shell-border), 0.48);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface), 0.72);
}

.admin-email-composer__step {
  display: flex;
  align-items: center;
  gap: 10px;
  color: rgb(var(--app-shell-text-secondary));
  opacity: 0.68;
}

.admin-email-composer__step--active {
  color: rgb(var(--app-shell-text));
  opacity: 1;
}

.admin-email-composer__step > div,
.admin-email-composer__continue > div {
  display: grid;
  gap: 2px;
}

.admin-email-composer__step span,
.admin-email-composer__continue span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.8rem;
}

.admin-email-composer__step-arrow {
  color: rgb(var(--app-shell-text-secondary));
}

.admin-email-composer__compose {
  max-width: 1040px;
  width: 100%;
  margin-inline: auto;
}

.admin-email-composer__body,
.admin-email-composer__message {
  display: grid;
  gap: 24px;
  padding: 24px;
}

.admin-email-composer__section-heading {
  display: flex;
  align-items: center;
  gap: 12px;
}

.admin-email-composer__section-heading h2,
.admin-email-composer__section-heading p {
  margin: 0;
}

.admin-email-composer__section-heading h2 {
  font-size: 1rem;
  font-weight: 800;
}

.admin-email-composer__section-heading p {
  margin-top: 2px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.875rem;
}

.admin-email-composer__continue {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 20px;
  border: 1px solid rgba(var(--app-shell-border), 0.48);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.42);
}

@media (max-width: 720px) {
  .admin-email-composer__steps {
    align-items: stretch;
    flex-direction: column;
  }

  .admin-email-composer__step-arrow {
    align-self: center;
    transform: rotate(90deg);
  }

  .admin-email-composer__continue {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
