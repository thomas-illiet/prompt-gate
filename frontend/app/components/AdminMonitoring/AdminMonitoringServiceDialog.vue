<script setup lang="ts">
import type {
  MonitoringService,
  MonitoringServicePayload,
} from '~/types/monitoring'

const props = defineProps<{
  loading: boolean
  service: MonitoringService | null
}>()

const emit = defineEmits<{
  save: [payload: MonitoringServicePayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const name = shallowRef('')
const displayName = shallowRef('')
const url = shallowRef('')
const expectedStatusCode = shallowRef(200)
const intervalSeconds = shallowRef(60)
const enabled = shallowRef(true)
const hasSubmitted = shallowRef(false)

const title = computed(() =>
  props.service ? 'Update monitoring service' : 'Create monitoring service',
)
const submitLabel = computed(() =>
  props.service ? 'Save service' : 'Create service',
)
const normalizedName = computed(() => name.value.trim().toLowerCase())
const trimmedURL = computed(() => url.value.trim())
const nameError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }

  if (!normalizedName.value) {
    return 'Name is required.'
  }

  if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(normalizedName.value)) {
    return 'Use lowercase letters, numbers, and single hyphens.'
  }

  return ''
})
const urlError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }

  if (!trimmedURL.value) {
    return 'URL is required.'
  }

  try {
    const parsed = new URL(trimmedURL.value)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      return 'Use an HTTP or HTTPS URL.'
    }
  } catch {
    return 'Use a valid HTTP or HTTPS URL.'
  }

  return ''
})
const expectedStatusCodeError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }
  if (expectedStatusCode.value < 100 || expectedStatusCode.value > 599) {
    return 'Use a status code between 100 and 599.'
  }
  return ''
})
const intervalSecondsError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }
  if (intervalSeconds.value < 30 || intervalSeconds.value > 86400) {
    return 'Use an interval between 30 and 86400 seconds.'
  }
  return ''
})
const canSave = computed(
  () =>
    !nameError.value &&
    !urlError.value &&
    !expectedStatusCodeError.value &&
    !intervalSecondsError.value &&
    Boolean(normalizedName.value),
)

watch(
  [isOpen, () => props.service],
  ([open]) => {
    if (!open) {
      return
    }

    name.value = props.service?.name ?? ''
    displayName.value = props.service?.displayName ?? ''
    url.value = props.service?.url ?? ''
    expectedStatusCode.value = props.service?.expectedStatusCode ?? 200
    intervalSeconds.value = props.service?.intervalSeconds ?? 60
    enabled.value = props.service?.enabled ?? true
    hasSubmitted.value = false
  },
  { immediate: true },
)

function save() {
  hasSubmitted.value = true

  if (!canSave.value) {
    return
  }

  emit('save', {
    name: normalizedName.value,
    displayName: displayName.value.trim(),
    url: trimmedURL.value,
    expectedStatusCode: expectedStatusCode.value,
    intervalSeconds: intervalSeconds.value,
    enabled: enabled.value,
  })
}
</script>

<template>
  <v-dialog v-model="isOpen" max-width="640" :persistent="props.loading">
    <v-card rounded="xl" class="admin-monitoring-dialog">
      <v-card-title class="pt-6 px-6 text-h6">
        {{ title }}
      </v-card-title>

      <form class="admin-monitoring-dialog__form" @submit.prevent="save">
        <v-card-text class="px-6 pb-2">
          <v-row>
            <v-col cols="12" md="6">
              <v-text-field
                v-model="name"
                label="Name"
                placeholder="api-health"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(nameError)"
                :error-messages="nameError ? [nameError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model="displayName"
                label="Display name"
                placeholder="API health"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
              />
            </v-col>

            <v-col cols="12">
              <v-text-field
                v-model="url"
                label="URL"
                placeholder="https://api.example.com/health"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(urlError)"
                :error-messages="urlError ? [urlError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model.number="expectedStatusCode"
                label="Expected HTTP code"
                type="number"
                variant="outlined"
                density="comfortable"
                min="100"
                max="599"
                :error="Boolean(expectedStatusCodeError)"
                :error-messages="
                  expectedStatusCodeError ? [expectedStatusCodeError] : []
                "
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model.number="intervalSeconds"
                label="Interval seconds"
                type="number"
                variant="outlined"
                density="comfortable"
                min="30"
                max="86400"
                :error="Boolean(intervalSecondsError)"
                :error-messages="
                  intervalSecondsError ? [intervalSecondsError] : []
                "
              />
            </v-col>

            <v-col cols="12">
              <v-checkbox
                v-model="enabled"
                label="Enabled"
                density="comfortable"
                hide-details
              />
            </v-col>
          </v-row>
        </v-card-text>

        <v-card-actions class="px-6 pb-6">
          <v-spacer />
          <AppDialogCloseButton label="Cancel" @click="isOpen = false" />
          <AppDialogActionButton
            color="primary"
            :label="submitLabel"
            type="submit"
            :loading="props.loading"
          />
        </v-card-actions>
      </form>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.admin-monitoring-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
}

.admin-monitoring-dialog__form {
  display: contents;
}
</style>
