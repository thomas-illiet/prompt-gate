import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppTokenCreateForm from '../../app/components/App/AppTokenCreateForm.vue'

function mountForm(props = {}) {
  return mount(AppTokenCreateForm, {
    props: {
      defaultLifetime: 30,
      loading: false,
      ...props,
    },
    global: {
      stubs: {
        AppDialogActionButton: {
          props: ['disabled', 'label', 'loading', 'type'],
          template:
            '<button data-test="submit" :disabled="disabled" :type="type">{{ label }}</button>',
        },
        VTextField: {
          emits: ['update:modelValue'],
          props: ['disabled', 'label', 'modelValue', 'type'],
          template:
            '<input :data-test="label" :disabled="disabled" :type="type || \'text\'" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
        },
      },
    },
  })
}

describe('AppTokenCreateForm', () => {
  it('keeps submit disabled until virtual key name is present', async () => {
    const wrapper = mountForm()

    expect(wrapper.get('[data-test="submit"]').attributes('disabled')).toBe('')

    await wrapper.get('[data-test="Virtual key name"]').setValue('personal_cli')

    expect(
      wrapper.get('[data-test="submit"]').attributes('disabled'),
    ).toBeUndefined()
  })

  it('emits trimmed virtual key payload with numeric lifetime within max lifetime', async () => {
    const wrapper = mountForm()

    await wrapper
      .get('[data-test="Virtual key name"]')
      .setValue(' personal_cli ')
    await wrapper.get('[data-test="Description"]').setValue(' CLI access ')
    await wrapper.get('[data-test="Lifetime"]').setValue('45')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('create')).toEqual([
      [
        {
          description: 'CLI access',
          expiresInDays: 45,
          name: 'personal_cli',
        },
      ],
    ])
  })

  it('disables submit when lifetime exceeds the configured max lifetime', async () => {
    const wrapper = mountForm({ maxLifetime: 30 })

    await wrapper.get('[data-test="Virtual key name"]').setValue('personal_cli')
    await wrapper.get('[data-test="Lifetime"]').setValue('31')

    expect(wrapper.get('[data-test="submit"]').attributes('disabled')).toBe('')

    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('create')).toBeUndefined()
  })
})
