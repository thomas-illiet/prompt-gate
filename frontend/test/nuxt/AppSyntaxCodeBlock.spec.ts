import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppSyntaxCodeBlock from '../../app/components/App/AppSyntaxCodeBlock.vue'

describe('AppSyntaxCodeBlock', () => {
  it('renders highlighted tokens with stable line numbers', () => {
    const wrapper = mount(AppSyntaxCodeBlock, {
      props: {
        code: '{\n  "model": "gpt-4.1",\n  "stream": true\n}',
        language: 'json',
      },
    })

    expect(wrapper.text()).toContain('JSON')
    expect(wrapper.findAll('[data-test="syntax-code-line"]')).toHaveLength(4)
    expect(wrapper.text()).toContain('01')
    expect(wrapper.text()).toContain('04')
    expect(wrapper.find('[class*="__token--"]').exists()).toBe(true)
  })

  it('escapes plaintext instead of creating executable elements', () => {
    const source = '<script>alert("unsafe")</script>'
    const wrapper = mount(AppSyntaxCodeBlock, {
      props: { code: source, language: 'unknown-language' },
    })

    expect(wrapper.text()).toContain(source)
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.text()).toContain('Plain text')
  })

  it('reacts to source and language changes', async () => {
    const wrapper = mount(AppSyntaxCodeBlock, {
      props: { code: 'echo first', language: 'bash' },
    })

    expect(wrapper.text()).toContain('Bash')
    await wrapper.setProps({
      code: 'from openai import OpenAI',
      language: 'python',
    })
    expect(wrapper.text()).toContain('Python')
    expect(wrapper.text()).toContain('from openai import OpenAI')
  })
})
