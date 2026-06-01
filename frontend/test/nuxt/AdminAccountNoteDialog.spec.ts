import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminAccountNoteDialog from '../../app/components/AdminAccounts/AdminAccountNoteDialog.vue'

const account = {
  email: 'ada@example.com',
  name: 'Ada Lovelace',
  note: 'Existing note',
  preferredUsername: 'ada',
}

function mountDialog() {
  return mount(AdminAccountNoteDialog, {
    props: {
      account,
      loading: false,
      modelValue: true,
    },
    global: {
      stubs: {
        AppDialogActionButton: {
          emits: ['click'],
          props: ['disabled', 'label', 'loading', 'type'],
          template:
            '<button data-test="save" :type="type || \'button\'" :disabled="disabled || loading" @click="$emit(\'click\')">{{ label }}</button>',
        },
        AppDialogCloseButton: {
          emits: ['click'],
          props: ['label'],
          template:
            '<button data-test="cancel" type="button" @click="$emit(\'click\')">{{ label }}</button>',
        },
        VAvatar: { template: '<span><slot /></span>' },
        VCard: { template: '<section><slot /></section>' },
        VCardActions: { template: '<div><slot /></div>' },
        VCardItem: {
          template: '<div><slot name="prepend" /><slot /></div>',
        },
        VCardSubtitle: { template: '<p><slot /></p>' },
        VCardText: { template: '<div><slot /></div>' },
        VCardTitle: { template: '<h2><slot /></h2>' },
        VDialog: {
          props: ['modelValue'],
          template: '<div v-if="modelValue"><slot /></div>',
        },
        VIcon: { template: '<i />' },
        VList: { template: '<div><slot /></div>' },
        VListItem: {
          props: ['subtitle', 'title'],
          template:
            '<div><span>{{ title }}</span><span>{{ subtitle }}</span></div>',
        },
        VSheet: { template: '<div><slot /></div>' },
        VSpacer: { template: '<span />' },
        VTextarea: {
          emits: ['update:modelValue'],
          props: ['modelValue'],
          template:
            '<textarea data-test="note" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
        },
      },
    },
  })
}

describe('AdminAccountNoteDialog', () => {
  it('initializes from the selected account and emits edited note text', async () => {
    const wrapper = mountDialog()

    expect(wrapper.text()).toContain('Ada Lovelace')
    expect(wrapper.text()).toContain('ada')
    expect(
      wrapper.get<HTMLTextAreaElement>('[data-test="note"]').element.value,
    ).toBe('Existing note')

    await wrapper.get('[data-test="note"]').setValue('Updated note')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toEqual([['Updated note']])
  })
})
