<script setup lang="ts">
import type { MCPServer, MCPServerPayload } from '~/types/mcp'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'MCP servers',
  icon: 'mdi-server-network-outline',
  drawerIndex: 8,
  drawerSection: 'Infrastructure',
})

const adminMCP = useAdminMCP()
const serverDialogOpen = shallowRef(false)
const deleteDialog = useTargetDialog<MCPServer>()
const toggleDialog = useTargetDialog<MCPServer>()
const toggleConfirm = useToggleConfirmDialog(toggleDialog.target, {
  disableIcon: 'mdi-server-network-off',
  enableIcon: 'mdi-server-network',
  entityLabel: 'MCP server',
  fallbackMessage: 'Change this MCP server status.',
  isActive: (server) => server.enabled,
  name: (server) => server.name,
})

const totalLabel = computed(() => {
  const total = adminMCP.servers.value.length
  const enabled = adminMCP.enabledServersCount.value
  return total === 1 ? `${enabled}/1 enabled` : `${enabled}/${total} enabled`
})

// openCreateDialog prepares a blank MCP server form.
function openCreateDialog() {
  adminMCP.selectedServer.value = null
  serverDialogOpen.value = true
}

// openEditDialog loads an MCP server before showing the edit dialog.
async function openEditDialog(server: MCPServer) {
  await adminMCP.loadServer(server.id)
  serverDialogOpen.value = true
}

// saveServer creates or updates the active MCP server form.
async function saveServer(payload: MCPServerPayload) {
  if (adminMCP.selectedServer.value) {
    await adminMCP.updateServer(adminMCP.selectedServer.value.id, payload)
  } else {
    await adminMCP.createServer(payload)
  }

  serverDialogOpen.value = false
}

// confirmToggleServer toggles the selected MCP server enabled state.
async function confirmToggleServer() {
  if (!toggleDialog.target.value) {
    return
  }

  const server = toggleDialog.target.value
  await adminMCP.updateServer(server.id, {
    name: server.name,
    displayName: server.displayName,
    url: server.url,
    headers: server.headers.map((header) => ({
      name: header.name,
      value: header.sensitive ? undefined : (header.value ?? ''),
      sensitive: header.sensitive,
    })),
    allowPattern: server.allowPattern,
    denyPattern: server.denyPattern,
    enabled: !server.enabled,
  })
  toggleDialog.close()
}

// confirmDelete removes the selected MCP server.
async function confirmDelete() {
  if (!deleteDialog.target.value) {
    return
  }

  await adminMCP.deleteServer(deleteDialog.target.value.id)
  deleteDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-server-network-outline"
          kicker="Runtime tools"
          title="MCP servers"
          copy="Manage streamable HTTP MCP servers and tool allow or deny filters used by proxy traffic."
          stat-label="Enabled servers"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="adminMCP.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminMCP.listError.value }}
        </v-alert>

        <AdminMcpTable
          :items="adminMCP.servers.value"
          :loading="adminMCP.loading.value"
          :page="adminMCP.page.value"
          :page-size="adminMCP.pageSize.value"
          :sort-by="adminMCP.sortBy.value"
          :sort-dir="adminMCP.sortDir.value"
          :total="adminMCP.total.value"
          @create="openCreateDialog"
          @delete="deleteDialog.open"
          @edit="openEditDialog"
          @refresh="adminMCP.reload"
          @toggle="toggleDialog.open"
          @update:page="adminMCP.setPage"
          @update:page-size="adminMCP.setPageSize"
          @update:sort="adminMCP.setSort"
        />
      </v-col>
    </v-row>

    <AdminMcpServerDialog
      v-model="serverDialogOpen"
      :loading="adminMCP.saving.value"
      :server="adminMCP.selectedServer.value"
      @save="saveServer"
    />

    <AppConfirmDialog
      v-model="deleteDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Delete server"
      icon="mdi-delete-alert-outline"
      :loading="adminMCP.saving.value"
      :message="
        deleteDialog.target.value
          ? `Delete MCP server ${deleteDialog.target.value.name}.`
          : 'Delete this MCP server.'
      "
      title="Delete MCP server"
      @cancel="deleteDialog.close"
      @confirm="confirmDelete"
    />

    <AppConfirmDialog
      v-model="toggleDialog.isOpen.value"
      :confirm-color="toggleConfirm.confirmColor.value"
      :confirm-label="toggleConfirm.actionLabel.value"
      :icon="toggleConfirm.icon.value"
      :loading="adminMCP.saving.value"
      :message="toggleConfirm.message.value"
      :title="toggleConfirm.title.value"
      @cancel="toggleDialog.close"
      @confirm="confirmToggleServer"
    />
  </v-container>
</template>
