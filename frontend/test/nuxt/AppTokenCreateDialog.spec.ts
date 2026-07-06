import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppTokenCreateDialog from '../../app/components/App/AppTokenCreateDialog.vue'

function mountDialog() {
  return mount(AppTokenCreateDialog, {
    props: {
      defaultLifetime: 365,
      loading: false,
      maxLifetime: 365,
      maxWidth: 700,
      modelValue: true,
      namePlaceholder: 'ci_token',
      submitIcon: 'mdi-key-plus',
      submitLabel: 'Create key',
      subtitle: 'Generate a new virtual key for this service account.',
      title: 'Create service account key',
    },
    global: {
      stubs: {
        AppDialogActionButton: {
          props: ['disabled', 'label', 'loading', 'prependIcon', 'type'],
          template:
            '<button data-test="submit" :data-icon="prependIcon" :disabled="disabled" :type="type">{{ label }}</button>',
        },
        AppDialogCard: {
          props: [
            'loading',
            'maxWidth',
            'modelValue',
            'subtitle',
            'title',
          ],
          template: `
            <section
              v-show="modelValue"
              data-test="dialog"
              :data-loading="String(loading)"
              :data-max-width="String(maxWidth)"
              :data-subtitle="subtitle"
              :data-title="title"
            >
              <slot />
              <slot name="actions" />
            </section>
          `,
        },
        AppDialogCloseButton: {
          template: '<button type="button">Close</button>',
        },
        VSpacer: { template: '<span />' },
        VTextField: {
          emits: ['update:modelValue'],
          props: [
            'disabled',
            'label',
            'max',
            'modelValue',
            'placeholder',
            'type',
          ],
          template:
            '<input :data-test="label" :disabled="disabled" :max="max" :placeholder="placeholder" :type="type || \'text\'" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
        },
      },
    },
  })
}

describe('AppTokenCreateDialog', () => {
  it('forwards dialog and form props, emits payloads, and resets on reopen', async () => {
    const wrapper = mountDialog()

    expect(wrapper.get('[data-test="dialog"]').attributes()).toMatchObject({
      'data-max-width': '700',
      'data-subtitle': 'Generate a new virtual key for this service account.',
      'data-title': 'Create service account key',
    })
    expect(wrapper.get('[data-test="Virtual key name"]').attributes()).toMatchObject(
      {
        placeholder: 'ci_token',
      },
    )
    expect(wrapper.get('[data-test="Lifetime"]').element).toHaveProperty(
      'value',
      '365',
    )
    expect(wrapper.get('[data-test="Lifetime"]').attributes('max')).toBe('365')
    expect(wrapper.get('[data-test="submit"]').attributes()).toMatchObject({
      'data-icon': 'mdi-key-plus',
    })
    expect(wrapper.get('[data-test="submit"]').text()).toBe('Create key')

    await wrapper.get('[data-test="Virtual key name"]').setValue(' ci_key ')
    await wrapper.get('[data-test="Description"]').setValue(' CI access ')
    await wrapper.get('[data-test="Lifetime"]').setValue('90')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('create')).toEqual([
      [
        {
          description: 'CI access',
          expiresInDays: 90,
          name: 'ci_key',
        },
      ],
    ])

    await wrapper.get('[data-test="Virtual key name"]').setValue('stale_key')
    await wrapper.setProps({ modelValue: false })
    await wrapper.setProps({ modelValue: true })

    expect(wrapper.get('[data-test="Virtual key name"]').element).toHaveProperty(
      'value',
      '',
    )
    expect(wrapper.get('[data-test="Lifetime"]').element).toHaveProperty(
      'value',
      '365',
    )
  })
})
