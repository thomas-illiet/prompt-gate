<script setup lang="ts">
import type { MCPHeaderFormRow } from '~/utils/mcp'

const props = defineProps<{
  errors: Record<string, string>
}>()

const rows = defineModel<MCPHeaderFormRow[]>({ required: true })

let rowId = 0

// createRow returns a new editable MCP header row.
function createRow(): MCPHeaderFormRow {
  rowId += 1
  return {
    id: `mcp-header-${Date.now()}-${rowId}`,
    name: '',
    value: '',
    sensitive: false,
    hasValue: false,
    clearValue: false,
  }
}

// addRow appends a blank MCP header row.
function addRow() {
  rows.value = [...rows.value, createRow()]
}

// removeRow deletes a header row by its local ID.
function removeRow(rowId: string) {
  rows.value = rows.value.filter((row) => row.id !== rowId)
}

// updateValue stores the cleartext header value and clears reset state.
function updateValue(row: MCPHeaderFormRow, value: string) {
  row.value = value
  row.clearValue = false
}

// clearStoredValue marks a persisted sensitive header for removal.
function clearStoredValue(row: MCPHeaderFormRow) {
  row.value = ''
  row.clearValue = true
}
</script>

<template>
  <div class="admin-mcp-headers">
    <div class="admin-mcp-headers__header">
      <div>
        <h3 class="admin-mcp-headers__title">Headers</h3>
        <p class="admin-mcp-headers__subtitle">
          Sensitive values stay redacted after save.
        </p>
      </div>

      <v-btn
        color="primary"
        variant="tonal"
        rounded="xl"
        prepend-icon="mdi-plus"
        @click="addRow"
      >
        Add header
      </v-btn>
    </div>

    <v-alert
      v-if="rows.length === 0"
      type="info"
      variant="tonal"
      rounded="lg"
      density="comfortable"
    >
      No custom headers configured.
    </v-alert>

    <div v-else class="admin-mcp-headers__rows">
      <div v-for="row in rows" :key="row.id" class="admin-mcp-headers__row">
        <v-text-field
          v-model="row.name"
          label="Header name"
          placeholder="Authorization"
          variant="outlined"
          density="comfortable"
          autocomplete="off"
          :error="Boolean(props.errors[row.id])"
          :error-messages="
            props.errors[row.id] ? [props.errors[row.id] ?? ''] : []
          "
        />

        <v-text-field
          :model-value="row.value"
          :label="
            row.sensitive && row.hasValue && !row.clearValue
              ? 'Header value stored'
              : 'Header value'
          "
          :placeholder="
            row.sensitive && row.hasValue && !row.clearValue
              ? 'Leave blank to keep stored value'
              : 'Value'
          "
          :type="row.sensitive ? 'password' : 'text'"
          variant="outlined"
          density="comfortable"
          autocomplete="off"
          @update:model-value="updateValue(row, String($event ?? ''))"
        >
          <template
            v-if="row.sensitive && row.hasValue && !row.value && !row.clearValue"
            #append-inner
          >
            <v-chip size="x-small" label color="success" variant="tonal">
              Stored
            </v-chip>
          </template>
        </v-text-field>

        <div class="admin-mcp-headers__controls">
          <v-switch
            v-model="row.sensitive"
            color="primary"
            inset
            hide-details
            label="Sensitive"
            @update:model-value="
              row.clearValue = row.sensitive ? row.clearValue : false
            "
          />

          <v-tooltip
            v-if="row.sensitive && row.hasValue && !row.clearValue"
            text="Clear stored value"
          >
            <template #activator="{ props: tooltipProps }">
              <v-btn
                v-bind="tooltipProps"
                aria-label="Clear stored header value"
                icon="mdi-lock-remove-outline"
                size="small"
                variant="text"
                color="warning"
                @click="clearStoredValue(row)"
              />
            </template>
          </v-tooltip>

          <v-tooltip text="Remove header">
            <template #activator="{ props: tooltipProps }">
              <v-btn
                v-bind="tooltipProps"
                aria-label="Remove header"
                icon="mdi-delete-outline"
                size="small"
                variant="text"
                color="error"
                @click="removeRow(row.id)"
              />
            </template>
          </v-tooltip>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.admin-mcp-headers {
  display: grid;
  gap: 14px;
}

.admin-mcp-headers__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.admin-mcp-headers__title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 700;
}

.admin-mcp-headers__subtitle {
  margin: 2px 0 0;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.86rem;
}

.admin-mcp-headers__rows {
  display: grid;
  gap: 12px;
}

.admin-mcp-headers__row {
  display: grid;
  grid-template-columns: minmax(160px, 0.9fr) minmax(220px, 1.2fr) auto;
  gap: 12px;
  align-items: start;
  padding: 14px;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface), 0.72);
}

.admin-mcp-headers__controls {
  min-height: 48px;
  display: flex;
  align-items: center;
  gap: 4px;
}

@media (max-width: 840px) {
  .admin-mcp-headers__header,
  .admin-mcp-headers__row {
    align-items: stretch;
    grid-template-columns: 1fr;
  }

  .admin-mcp-headers__header {
    display: grid;
  }

  .admin-mcp-headers__controls {
    justify-content: space-between;
  }
}
</style>
