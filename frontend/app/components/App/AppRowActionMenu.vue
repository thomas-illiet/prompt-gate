<script setup lang="ts" generic="TItem">
import type {
  AppRowAction,
  AppRowActionSelectContext,
} from '~/types/row-actions'

const props = withDefaults(
  defineProps<{
    actions: AppRowAction<TItem>[]
    ariaLabel?: string
    item?: TItem
    itemLabel?: ((item: TItem) => string) | string
    label?: string
    minWidth?: number | string
  }>(),
  {
    ariaLabel: 'Open actions',
    item: undefined,
    itemLabel: undefined,
    label: 'Actions',
    minWidth: 180,
  },
)

const emit = defineEmits<{
  select: [key: string, context: AppRowActionSelectContext<TItem>]
}>()

const accessibleLabel = computed(() => {
  if (props.ariaLabel !== 'Open actions') {
    return props.ariaLabel
  }

  if (props.item === undefined) {
    return 'Open actions'
  }

  if (typeof props.itemLabel === 'function') {
    return `Open actions for ${props.itemLabel(props.item)}`
  }

  if (typeof props.itemLabel === 'string' && props.itemLabel.trim()) {
    return `Open actions for ${props.itemLabel}`
  }

  if (typeof props.item === 'object' && props.item !== null) {
    const record = props.item as Record<string, unknown>
    for (const key of ['name', 'title', 'address', 'model', 'id']) {
      const value = record[key]
      if (typeof value === 'string' && value.trim()) {
        return `Open actions for ${value}`
      }
    }
  }

  return 'Open actions for this row'
})

// resolveActionTitle evaluates a static or item-derived action title.
function resolveActionTitle(action: AppRowAction<TItem>) {
  if (typeof action.title === 'function') {
    return props.item === undefined ? '' : action.title(props.item)
  }

  return action.title
}

// isActionDisabled evaluates an action disabled predicate for the current row.
function isActionDisabled(action: AppRowAction<TItem>) {
  if (typeof action.disabled === 'function') {
    return props.item === undefined ? true : action.disabled(props.item)
  }

  return action.disabled ?? false
}

// selectAction emits the selected row action when it is enabled.
function selectAction(action: AppRowAction<TItem>) {
  if (isActionDisabled(action)) {
    return
  }

  const context = {
    action,
    item: props.item,
  }

  if (props.item !== undefined) {
    action.onSelect?.(props.item)
  }

  emit('select', action.key, context)
}
</script>

<template>
  <div class="app-row-action-menu">
    <v-menu location="bottom end" offset="8">
      <template #activator="{ props: menuProps }">
        <v-btn
          v-bind="menuProps"
          size="small"
          variant="outlined"
          rounded="lg"
          class="app-row-action-menu__button text-none"
          :aria-label="accessibleLabel"
        >
          {{ props.label }}
          <template #append>
            <v-icon
              icon="mdi-chevron-down"
              size="18"
              class="text-medium-emphasis"
            />
          </template>
        </v-btn>
      </template>

      <v-list density="comfortable" :min-width="props.minWidth">
        <v-list-item
          v-for="action in props.actions"
          :key="action.key"
          :prepend-icon="action.icon"
          :title="resolveActionTitle(action)"
          :base-color="action.color"
          :disabled="isActionDisabled(action)"
          @click="selectAction(action)"
        />
      </v-list>
    </v-menu>
  </div>
</template>

<style scoped>
.app-row-action-menu {
  display: flex;
  align-items: center;
  justify-content: center;
}

.app-row-action-menu__button {
  min-width: 0;
}
</style>
