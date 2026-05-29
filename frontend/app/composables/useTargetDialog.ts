// useTargetDialog tracks a dialog's open state and selected target.
export function useTargetDialog<T>() {
  const isOpen = shallowRef(false)
  const target = shallowRef<T | null>(null)

  // open stores the target and shows the dialog.
  function open(nextTarget: T) {
    target.value = nextTarget
    isOpen.value = true
  }

  // close hides the dialog and clears the target.
  function close() {
    isOpen.value = false
    target.value = null
  }

  return {
    close,
    isOpen,
    open,
    target,
  }
}
