import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppDialogCard from '../../app/components/App/AppDialogCard.vue'

function mountDialog(props = {}) {
  return mount(AppDialogCard, {
    props: {
      modelValue: true,
      subtitle: 'Helpful context',
      title: 'Edit provider',
      ...props,
    },
    slots: {
      actions: '<button data-test="action">Save</button>',
      default: '<p data-test="content">Dialog content</p>',
    },
    global: {
      stubs: {
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: {
          emits: ['click'],
          props: ['ariaLabel', 'disabled'],
          template:
            '<button data-test="close" :aria-label="ariaLabel" :disabled="disabled" @click="$emit(\'click\')" />',
        },
        VCard: { template: '<section><slot /></section>' },
        VCardActions: { template: '<footer><slot /></footer>' },
        VCardItem: {
          template:
            '<header><slot name="prepend" /><slot /><slot name="append" /></header>',
        },
        VCardSubtitle: { template: '<p><slot /></p>' },
        VCardText: { template: '<main><slot /></main>' },
        VCardTitle: { template: '<h2><slot /></h2>' },
        VDialog: {
          props: [
            'ariaDescribedby',
            'ariaLabelledby',
            'maxWidth',
            'modelValue',
            'persistent',
            'scrollable',
          ],
          template:
            '<div data-test="dialog" :aria-labelledby="ariaLabelledby" :aria-describedby="ariaDescribedby" :data-persistent="persistent" :data-scrollable="scrollable"><slot /></div>',
        },
        VIcon: { template: '<i />' },
      },
    },
  })
}

describe('AppDialogCard', () => {
  it('connects the dialog to its title and description', () => {
    const wrapper = mountDialog()
    const dialog = wrapper.get('[data-test="dialog"]')
    const title = wrapper.get('h2')
    const subtitle = wrapper.get('p')

    expect(dialog.attributes('aria-labelledby')).toBe(title.attributes('id'))
    expect(dialog.attributes('aria-describedby')).toBe(subtitle.attributes('id'))
    expect(dialog.attributes('data-scrollable')).toBeDefined()
    expect(wrapper.get('[data-test="content"]').text()).toBe('Dialog content')
    expect(wrapper.get('[data-test="action"]').text()).toBe('Save')
  })

  it('closes from the accessible close button when idle', async () => {
    const wrapper = mountDialog()
    const closeButton = wrapper.get('[data-test="close"]')

    expect(closeButton.attributes('aria-label')).toBe('Close dialog')
    await closeButton.trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[false]])
  })

  it('disables dismissal while loading', async () => {
    const wrapper = mountDialog({ loading: true })
    const closeButton = wrapper.get('[data-test="close"]')

    expect(closeButton.attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="dialog"]').attributes('data-persistent')).toBe(
      'true',
    )
  })
})
