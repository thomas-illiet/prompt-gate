import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminSetupGuidePreview from '../../app/components/AdminSetupGuides/AdminSetupGuidePreview.vue'
import type { SetupGuidePayload } from '../../app/types/setup-guides'

const guide: SetupGuidePayload = {
  identifier: 'python',
  title: 'Python',
  subtitle: 'Python SDK',
  icon: 'mdi-language-python',
  compatibility: 'openai',
  modelMode: 'single',
  syntaxLanguage: 'python',
  filePaths: [],
  template: 'print("{{model}}")',
  enabled: true,
  position: 0,
}

describe('AdminSetupGuidePreview', () => {
  it('renders the sample with the guide syntax language', () => {
    const wrapper = mount(AdminSetupGuidePreview, {
      props: { guide },
      global: {
        stubs: {
          AppSyntaxCodeBlock: {
            props: ['code', 'language'],
            template:
              '<pre data-test="syntax-preview" :data-language="language">{{ code }}</pre>',
          },
          VAlert: { template: '<div><slot /></div>' },
          VIcon: true,
        },
      },
    })

    const preview = wrapper.get('[data-test="syntax-preview"]')
    expect(preview.attributes('data-language')).toBe('python')
    expect(preview.text()).toContain('print("gpt-example")')
  })
})
