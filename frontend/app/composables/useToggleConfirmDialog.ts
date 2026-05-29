import type { Ref } from 'vue'

interface ToggleConfirmDialogOptions<TItem> {
  disableIcon: string
  enableIcon: string
  entityLabel: string
  fallbackMessage: string
  isActive: (item: TItem) => boolean
  name: (item: TItem) => string
}

// useToggleConfirmDialog derives labels and styling for enable/disable dialogs.
export function useToggleConfirmDialog<TItem>(
  target: Ref<TItem | null>,
  options: ToggleConfirmDialogOptions<TItem>,
) {
  const actionLabel = computed(() =>
    target.value && options.isActive(target.value) ? 'Disable' : 'Enable',
  )
  const confirmColor = computed(() =>
    target.value && options.isActive(target.value) ? 'warning' : 'success',
  )
  const icon = computed(() =>
    target.value && options.isActive(target.value)
      ? options.disableIcon
      : options.enableIcon,
  )
  const message = computed(() => {
    if (!target.value) {
      return options.fallbackMessage
    }

    const action = options.isActive(target.value) ? 'disable' : 'enable'
    return `Confirm ${action} for ${options.entityLabel} ${options.name(target.value)}.`
  })
  const title = computed(() => `${actionLabel.value} ${options.entityLabel}`)

  return {
    actionLabel,
    confirmColor,
    icon,
    message,
    title,
  }
}
