<script setup lang="ts">
import type { MCPServer, MCPServerPayload } from '~/types/mcp'
import type { MCPHeaderFormRow } from '~/utils/mcp'
import {
  buildMCPServerPayload,
  findDuplicateMCPHeaderNames,
  isValidMCPHeaderName,
  isValidMCPServerName,
  isValidMCPURL,
  normalizeMCPHeaderName,
  normalizeMCPServerName,
} from '~/utils/mcp'

const props = defineProps<{
  loading: boolean
  server: MCPServer | null
}>()

const emit = defineEmits<{
  save: [payload: MCPServerPayload]
}>()

const isOpen = defineModel<boolean>({ default: false })

const name = shallowRef('')
const displayName = shallowRef('')
const url = shallowRef('')
const allowPattern = shallowRef('')
const denyPattern = shallowRef('')
const enabled = shallowRef(true)
const hasSubmitted = shallowRef(false)
const headers = ref<MCPHeaderFormRow[]>([])

let headerId = 0

const title = computed(() =>
  props.server ? 'Update MCP server' : 'Create MCP server',
)
const submitLabel = computed(() =>
  props.server ? 'Save server' : 'Create server',
)
const duplicateHeaderNames = computed(() =>
  findDuplicateMCPHeaderNames(headers.value),
)
const nameError = computed(() => {
  if (!hasSubmitted.value || isValidMCPServerName(name.value)) {
    return ''
  }

  return 'Use lowercase letters, numbers, and hyphens.'
})
const urlError = computed(() => {
  if (!hasSubmitted.value || isValidMCPURL(url.value)) {
    return ''
  }

  return 'Use a valid HTTP or HTTPS URL.'
})
const allowPatternError = computed(() => regexError(allowPattern.value))
const denyPatternError = computed(() => regexError(denyPattern.value))
const headerErrors = computed<Record<string, string>>(() => {
  if (!hasSubmitted.value) {
    return {}
  }

  return Object.fromEntries(
    headers.value
      .map((header) => [header.id, headerError(header)] as const)
      .filter(([, error]) => Boolean(error)),
  )
})
const canSave = computed(
  () =>
    isValidMCPServerName(name.value) &&
    isValidMCPURL(url.value) &&
    !regexError(allowPattern.value) &&
    !regexError(denyPattern.value) &&
    headers.value.every((header) => !headerError(header)),
)

watch(
  [isOpen, () => props.server],
  ([open]) => {
    if (!open) {
      return
    }

    name.value = props.server?.name ?? ''
    displayName.value = props.server?.displayName ?? ''
    url.value = props.server?.url ?? ''
    allowPattern.value = props.server?.allowPattern ?? ''
    denyPattern.value = props.server?.denyPattern ?? ''
    enabled.value = props.server?.enabled ?? true
    headers.value =
      props.server?.headers.map((header) => ({
        id: nextHeaderId(),
        name: header.name,
        value: header.value ?? '',
        sensitive: header.sensitive,
        hasValue: header.hasValue,
        clearValue: false,
      })) ?? []
    hasSubmitted.value = false
  },
  { immediate: true },
)

// nextHeaderId creates a stable local ID for editable header rows.
function nextHeaderId() {
  headerId += 1
  return `mcp-server-header-${Date.now()}-${headerId}`
}

// regexError validates optional allow and deny patterns.
function regexError(value: string) {
  if (!hasSubmitted.value || !value.trim()) {
    return ''
  }

  try {
    new RegExp(value.trim())
    return ''
  } catch {
    return 'Use a valid regular expression.'
  }
}

// headerError returns validation feedback for one header row.
function headerError(header: MCPHeaderFormRow) {
  const normalizedName = normalizeMCPHeaderName(header.name)

  if (!isValidMCPHeaderName(normalizedName)) {
    return 'Header name is required and cannot contain spaces or colons.'
  }

  if (duplicateHeaderNames.value.has(normalizedName.toLowerCase())) {
    return 'Header names must be unique.'
  }

  return ''
}

// updateName normalizes the MCP server name as the user types.
function updateName(value: string) {
  name.value = normalizeMCPServerName(value)
}

// save validates the form and emits the MCP server payload.
function save() {
  hasSubmitted.value = true

  if (!canSave.value) {
    return
  }

  emit(
    'save',
    buildMCPServerPayload({
      name: name.value,
      displayName: displayName.value,
      url: url.value,
      headers: headers.value,
      allowPattern: allowPattern.value,
      denyPattern: denyPattern.value,
      enabled: enabled.value,
    }),
  )
}
</script>

<template>
  <v-dialog v-model="isOpen" max-width="920" :persistent="props.loading">
    <v-card rounded="xl" class="admin-mcp-dialog">
      <v-card-title class="pt-6 px-6 text-h6">
        {{ title }}
      </v-card-title>

      <form class="admin-mcp-dialog__form" @submit.prevent="save">
        <v-card-text class="px-6 pb-2">
          <v-row>
            <v-col cols="12" md="6">
              <v-text-field
                :model-value="name"
                label="Server name"
                placeholder="linear-tools"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(nameError)"
                :error-messages="nameError ? [nameError] : []"
                @update:model-value="updateName(String($event ?? ''))"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model="displayName"
                label="Display name"
                placeholder="Linear tools"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
              />
            </v-col>

            <v-col cols="12">
              <v-text-field
                v-model="url"
                label="Server URL"
                placeholder="https://mcp.example.com/mcp"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(urlError)"
                :error-messages="urlError ? [urlError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model="allowPattern"
                label="Allow tool pattern"
                placeholder="^jira_"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(allowPatternError)"
                :error-messages="allowPatternError ? [allowPatternError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model="denyPattern"
                label="Deny tool pattern"
                placeholder="delete|destroy"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(denyPatternError)"
                :error-messages="denyPatternError ? [denyPatternError] : []"
              />
            </v-col>

            <v-col cols="12">
              <AdminMcpHeaderRows v-model="headers" :errors="headerErrors" />
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
.admin-mcp-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
}

.admin-mcp-dialog__form {
  display: contents;
}
</style>
