<script setup lang="ts">
import type { SetupGuide } from '~/types/setup-guides'
const props = defineProps<{ guides: SetupGuide[]; loading: boolean }>()
const emit = defineEmits<{
  edit: [guide: SetupGuide]
  remove: [guide: SetupGuide]
  reorder: [ids: string[]]
}>()
function move(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= props.guides.length) return
  const ids = props.guides.map((g) => g.id)
  ;[ids[index], ids[target]] = [ids[target]!, ids[index]!]
  emit('reorder', ids)
}
</script>
<template>
  <v-table>
    <thead>
      <tr>
        <th>Order</th>
        <th>Guide</th>
        <th>Compatibility</th>
        <th>Models</th>
        <th>Status</th>
        <th class="text-right">Actions</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="(guide, index) in guides" :key="guide.id">
        <td>
          <v-btn
            icon="mdi-chevron-up"
            size="small"
            variant="text"
            :disabled="index === 0"
            @click="move(index, -1)"
          /><v-btn
            icon="mdi-chevron-down"
            size="small"
            variant="text"
            :disabled="index === guides.length - 1"
            @click="move(index, 1)"
          />
        </td>
        <td>
          <div class="d-flex align-center ga-2">
            <v-icon :icon="guide.icon" />
            <div>
              <strong>{{ guide.title }}</strong>
              <div class="text-caption">{{ guide.identifier }}</div>
            </div>
          </div>
        </td>
        <td>{{ guide.compatibility }}</td>
        <td>{{ guide.modelMode }}</td>
        <td>
          <v-chip :color="guide.enabled ? 'success' : 'default'" size="small">{{
            guide.enabled ? 'Enabled' : 'Disabled'
          }}</v-chip>
        </td>
        <td class="text-right">
          <v-btn
            icon="mdi-pencil-outline"
            variant="text"
            @click="emit('edit', guide)"
          /><v-btn
            color="error"
            icon="mdi-delete-outline"
            variant="text"
            @click="emit('remove', guide)"
          />
        </td>
      </tr>
    </tbody>
  </v-table>
  <AppEmptyState
    v-if="!loading && !guides.length"
    icon="mdi-file-code-outline"
    title="No setup guides"
    text="Create the first client guide."
  />
</template>
