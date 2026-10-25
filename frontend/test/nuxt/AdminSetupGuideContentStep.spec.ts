import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminSetupGuideContentStep from '../../app/components/AdminSetupGuides/AdminSetupGuideContentStep.vue'
import type { SetupGuidePayload } from '../../app/types/setup-guides'

const guide: SetupGuidePayload = {
  identifier: 'python',
  title: 'Python',
  subtitle: 'Python SDK',
  icon: 'mdi-language-python',
  compatibility: 'openai',
  modelMode: 'single',
  syntaxLanguage: 'plaintext',
  filePaths: [],
  template: 'print("{{model}}")',
  enabled: true,
  position: 0,
}

describe('AdminSetupGuideContentStep', () => {
  it('offers the supported syntax languages and updates the selection', async () => {
    const wrapper = mount(AdminSetupGuideContentStep, {
      props: {
        guide,
        templateError: null,
        filePathsText: '',
        syntaxLanguage: 'plaintext',
        template: guide.template,
      },
      global: {
        stubs: {
          AdminSetupGuidePreview: true,
          VChip: true,
          VIcon: true,
          VSelect: {
            name: 'VSelect',
            props: ['items', 'modelValue'],
            emits: ['update:modelValue'],
            template: '<div data-test="syntax-language" />',
          },
          VTab: { template: '<button><slot /></button>' },
          VTabs: { template: '<div><slot /></div>' },
          VTextarea: true,
        },
      },
    })

    const select = wrapper.getComponent({ name: 'VSelect' })
    expect(select.props('items')).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ title: 'Plain text', value: 'plaintext' }),
        expect.objectContaining({ title: 'Python', value: 'python' }),
        expect.objectContaining({ title: 'JSON5', value: 'json5' }),
      ]),
    )

    await select.vm.$emit('update:modelValue', 'python')
    expect(wrapper.emitted('update:syntaxLanguage')).toEqual([['python']])
  })
})
