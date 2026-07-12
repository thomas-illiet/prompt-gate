import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import FAQList from '../../app/components/FAQ/FAQList.vue'

describe('FAQList', () => {
  it('filters questions and renders sanitized backend HTML', async () => {
    const wrapper = mount(FAQList, {
      props: {
        entries: [
          { id: '1', question: 'How do I authenticate?', renderedHtml: '<p>Use a <strong>key</strong>.</p>', position: 0 },
          { id: '2', question: 'Where is usage?', renderedHtml: '<p>Dashboard.</p>', position: 1 },
        ],
        loading: false,
      },
      global: {
        stubs: {
          AppSectionCard: { template: '<section><slot /></section>' },
          VExpansionPanels: { template: '<div><slot /></div>' },
          VExpansionPanel: { template: '<article><slot /></article>' },
          VExpansionPanelTitle: { template: '<h2><slot /></h2>' },
          VExpansionPanelText: { template: '<div><slot /></div>' },
          VChip: { template: '<span><slot /></span>' },
          VTextField: { props: ['modelValue'], emits: ['update:modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
        },
      },
    })

    expect(wrapper.html()).toContain('<strong>key</strong>')
    await wrapper.get('input').setValue('usage')
    expect(wrapper.text()).not.toContain('authenticate')
    expect(wrapper.text()).toContain('Where is usage?')
  })
})
