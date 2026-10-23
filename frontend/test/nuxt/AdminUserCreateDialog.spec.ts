import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminUserCreateDialog from '../../app/components/AdminUsers/AdminUserCreateDialog.vue'

function mountDialog(options: { loading?: boolean } = {}) {
  return mount(AdminUserCreateDialog, {
    props: {
      loading: options.loading ?? false,
      modelValue: true,
    },
    global: {
      stubs: {
        AppDialogCard: {
          props: ['modelValue', 'subtitle'],
          template:
            '<section v-if="modelValue"><p>{{ subtitle }}</p><slot /><footer><slot name="actions" /></footer></section>',
        },
        AppDialogActionButton: {
          props: ['disabled', 'form', 'label', 'loading', 'type'],
          template:
            '<button :disabled="disabled || loading" :form="form" :type="type">{{ label }}</button>',
        },
        AppDialogCloseButton: {
          props: ['disabled', 'label'],
          template: '<button type="button" :disabled="disabled">{{ label }}</button>',
        },
        VTextField: {
          emits: ['update:modelValue'],
          props: [
            'disabled',
            'errorMessages',
            'label',
            'modelValue',
          ],
          template: `
            <label>
              {{ label }}
              <input
                data-test="user-uid"
                :disabled="disabled"
                :value="modelValue"
                @input="$emit('update:modelValue', $event.target.value)"
              />
              <span v-for="message in errorMessages" :key="message">{{ message }}</span>
            </label>
          `,
        },
      },
    },
  })
}

describe('AdminUserCreateDialog', () => {
  it('explains that pre-creation immediately grants limited access', () => {
    const wrapper = mountDialog()

    expect(wrapper.text()).toContain('Create an active limited account now.')
  })

  it('requires a UID before emitting', async () => {
    const wrapper = mountDialog()

    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('create')).toBeUndefined()
    expect(wrapper.text()).toContain('User UID is required.')
  })

  it('trims surrounding whitespace while preserving UID case', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="user-uid"]').setValue('  Alice.Example  ')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('create')).toEqual([
      [{ preferredUsername: 'Alice.Example' }],
    ])
  })

  it('resets its form whenever it is opened again', async () => {
    const wrapper = mountDialog()
    await wrapper.get('[data-test="user-uid"]').setValue('alice')

    await wrapper.setProps({ modelValue: false })
    await wrapper.setProps({ modelValue: true })

    expect(wrapper.get<HTMLInputElement>('[data-test="user-uid"]').element.value).toBe(
      '',
    )
  })

  it('disables the UID field and blocks duplicate submit while creation is in progress', async () => {
    const wrapper = mountDialog({ loading: true })

    expect(
      wrapper.get('[data-test="user-uid"]').attributes('disabled'),
    ).toBeDefined()
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('create')).toBeUndefined()
  })
})
