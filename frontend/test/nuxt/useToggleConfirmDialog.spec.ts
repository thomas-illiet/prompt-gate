import { describe, expect, it } from 'vitest'

import { useToggleConfirmDialog } from '../../app/composables/useToggleConfirmDialog'

interface TestTarget {
  enabled: boolean
  name: string
}

describe('useToggleConfirmDialog', () => {
  it('builds disable copy for active targets', () => {
    const target = shallowRef<TestTarget>({ enabled: true, name: 'openai' })
    const dialog = useToggleConfirmDialog(target, {
      disableIcon: 'mdi-off',
      enableIcon: 'mdi-on',
      entityLabel: 'provider',
      fallbackMessage: 'Change status.',
      isActive: (item) => item.enabled,
      name: (item) => item.name,
    })

    expect(dialog.actionLabel.value).toBe('Disable')
    expect(dialog.confirmColor.value).toBe('warning')
    expect(dialog.icon.value).toBe('mdi-off')
    expect(dialog.message.value).toBe('Confirm disable for provider openai.')
    expect(dialog.title.value).toBe('Disable provider')
  })

  it('builds enable copy and fallback message', () => {
    const target = shallowRef<TestTarget | null>({
      enabled: false,
      name: 'ollama',
    })
    const dialog = useToggleConfirmDialog(target, {
      disableIcon: 'mdi-off',
      enableIcon: 'mdi-on',
      entityLabel: 'provider',
      fallbackMessage: 'Change status.',
      isActive: (item) => item.enabled,
      name: (item) => item.name,
    })

    expect(dialog.actionLabel.value).toBe('Enable')
    expect(dialog.confirmColor.value).toBe('success')
    expect(dialog.icon.value).toBe('mdi-on')
    expect(dialog.message.value).toBe('Confirm enable for provider ollama.')

    target.value = null

    expect(dialog.message.value).toBe('Change status.')
  })
})
