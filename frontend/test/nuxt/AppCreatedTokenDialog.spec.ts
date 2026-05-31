import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AppCreatedTokenDialog from '../../app/components/App/AppCreatedTokenDialog.vue'

const { notifySuccess } = vi.hoisted(() => ({
  notifySuccess: vi.fn(),
}))

vi.mock('~/stores/notification', () => ({
  Notify: {
    success: notifySuccess,
  },
}))

describe('AppCreatedTokenDialog', () => {
  beforeEach(() => {
    notifySuccess.mockClear()
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    })
  })

  it('renders token metadata and copies raw token', async () => {
    const wrapper = mount(AppCreatedTokenDialog, {
      props: {
        createdToken: {
          token: 'raw.token.value',
          tokenInfo: {
            name: 'personal_cli',
            expiresAt: '2099-12-31T00:00:00Z',
          },
        },
        modelValue: true,
      },
      global: {
        stubs: {
          AppDialogActionButton: {
            emits: ['click'],
            props: ['label'],
            template:
              '<button data-test="copy" @click="$emit(\'click\')">{{ label }}</button>',
          },
          AppDialogCard: {
            name: 'AppDialogCard',
            props: ['icon', 'iconColor'],
            template:
              '<section><slot /><footer><slot name="actions" /></footer></section>',
          },
          AppDialogCloseButton: { template: '<button />' },
          VSpacer: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('raw.token.value')
    expect(wrapper.text()).toContain('personal_cli')
    expect(wrapper.getComponent({ name: 'AppDialogCard' }).props()).toMatchObject(
      {
        icon: 'mdi-key-plus',
        iconColor: 'success',
      },
    )

    await wrapper.get('[data-test="copy"]').trigger('click')

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      'raw.token.value',
    )
    expect(notifySuccess).toHaveBeenCalledWith('Virtual key copied.')
  })
})
