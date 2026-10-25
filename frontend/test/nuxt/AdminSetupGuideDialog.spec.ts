import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminSetupGuideDialog from '../../app/components/AdminSetupGuides/AdminSetupGuideDialog.vue'
import type { SetupGuide } from '../../app/types/setup-guides'

const guide: SetupGuide = {
  id: 'guide-id',
  identifier: 'python',
  title: 'Python',
  subtitle: 'Python SDK',
  icon: 'mdi-language-python',
  compatibility: 'openai',
  modelMode: 'single',
  syntaxLanguage: 'python',
  filePaths: ['main.py'],
  template: 'print("{{model}}")',
  enabled: true,
  position: 0,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

describe('AdminSetupGuideDialog', () => {
  it('loads and submits the explicit syntax language', async () => {
    const wrapper = mount(AdminSetupGuideDialog, {
      props: {
        guide,
        modelValue: false,
        nextPosition: 1,
        saving: false,
      },
      global: {
        stubs: {
          AdminSetupGuideContentStep: {
            name: 'AdminSetupGuideContentStep',
            props: [
              'filePathsText',
              'guide',
              'syntaxLanguage',
              'template',
              'templateError',
            ],
            emits: [
              'update:filePathsText',
              'update:syntaxLanguage',
              'update:template',
            ],
            template:
              '<button data-test="set-json5" @click="$emit(\'update:syntaxLanguage\', \'json5\')">{{ syntaxLanguage }}</button>',
          },
          AdminSetupGuideSettingsStep: true,
          AppDialogActionButton: {
            props: ['label'],
            emits: ['click'],
            template:
              '<button :data-label="label" @click="$emit(\'click\')">{{ label }}</button>',
          },
          AppDialogCard: {
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: '<section><slot /><slot name="actions" /></section>',
          },
          AppDialogCloseButton: true,
        },
      },
    })

    await wrapper.setProps({ modelValue: true })
    await wrapper.get('[data-label="Continue"]').trigger('click')

    const content = wrapper.getComponent({
      name: 'AdminSetupGuideContentStep',
    })
    expect(content.props('syntaxLanguage')).toBe('python')

    await wrapper.get('[data-test="set-json5"]').trigger('click')
    await wrapper.get('[data-label="Save changes"]').trigger('click')

    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({
      identifier: 'python',
      syntaxLanguage: 'json5',
      filePaths: ['main.py'],
    })
  })
})
