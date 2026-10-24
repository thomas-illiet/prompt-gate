import { describe, expect, it } from 'vitest'

import { renderModelLabMarkdown } from '../../app/utils/model-lab-markdown'

describe('Model Lab Markdown rendering', () => {
  it('renders useful Markdown and hardens external links', () => {
    const html = renderModelLabMarkdown(
      '# Answer\n\n**Safe** [docs](https://example.test)\n\n```ts\nconst ok = true\n```',
    )

    expect(html).toContain('Answer')
    expect(html).toContain('<strong>Safe</strong>')
    expect(html).toContain('href="https://example.test"')
    expect(html).toContain('rel="noopener noreferrer"')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('<code')
  })

  it('removes executable markup, tracking images and unsafe URLs', () => {
    const html = renderModelLabMarkdown(
      '<script>alert(1)</script><img src="https://tracker.test/pixel" onerror="alert(2)"> [bad](javascript:alert(3))',
    )

    expect(html).not.toContain('<script')
    expect(html).not.toContain('<img')
    expect(html).not.toContain('onerror=')
    expect(html).not.toContain('javascript:alert')
    expect(html).not.toContain('href=')
  })

  it('renders raw HTML as inert text before sanitizing Markdown output', () => {
    const html = renderModelLabMarkdown(
      '<strong data-danger="true">raw HTML</strong>',
    )

    expect(html).not.toContain('<strong')
    expect(html).not.toContain('data-danger')
    expect(html).toContain('raw HTML')
  })
})
