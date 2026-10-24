import DOMPurify from 'dompurify'
import { marked } from 'marked'

const ALLOWED_TAGS = [
  'a',
  'blockquote',
  'br',
  'code',
  'del',
  'em',
  'h1',
  'h2',
  'h3',
  'h4',
  'hr',
  'li',
  'ol',
  'p',
  'pre',
  'strong',
  'table',
  'tbody',
  'td',
  'th',
  'thead',
  'tr',
  'ul',
]
const ALLOWED_TAG_NAMES = new Set(ALLOWED_TAGS.map((tag) => tag.toUpperCase()))

export function escapeModelLabText(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

function plainTextFallback(content: string) {
  return `<pre class="model-lab-plain-text">${escapeModelLabText(content)}</pre>`
}

const renderer = new marked.Renderer()
renderer.html = () => ''

// renderModelLabMarkdown is the sole boundary allowed to turn provider text
// into HTML. It uses an explicit tag/attribute allowlist and never permits
// images, embeds, inline styles or executable URL schemes.
export function renderModelLabMarkdown(content: string) {
  if (typeof document === 'undefined' || typeof DOMPurify.sanitize !== 'function') {
    return plainTextFallback(content)
  }

  try {
    const rendered = marked.parse(content, {
      async: false,
      breaks: true,
      gfm: true,
      renderer,
    })
    const sanitized = DOMPurify.sanitize(rendered, {
      ALLOW_DATA_ATTR: false,
      ALLOWED_ATTR: ['href', 'title'],
      ALLOWED_TAGS,
      FORBID_ATTR: ['style'],
      FORBID_TAGS: ['audio', 'embed', 'iframe', 'img', 'math', 'object', 'script', 'style', 'svg', 'video'],
    })
    const template = document.createElement('template')
    template.innerHTML = sanitized

    for (const element of template.content.querySelectorAll('*')) {
      if (!ALLOWED_TAG_NAMES.has(element.tagName)) {
        element.replaceWith(document.createTextNode(element.textContent ?? ''))
        continue
      }
      for (const attribute of [...element.attributes]) {
        if (
          element.tagName !== 'A' ||
          (attribute.name !== 'href' && attribute.name !== 'title')
        ) {
          element.removeAttribute(attribute.name)
        }
      }
      if (element.tagName !== 'A') continue

      const href = element.getAttribute('href')?.trim() ?? ''
      if (!/^(?:https?:|mailto:)/i.test(href)) {
        element.removeAttribute('href')
        element.removeAttribute('title')
        continue
      }
      element.setAttribute('rel', 'noopener noreferrer')
      if (/^https?:/i.test(href)) element.setAttribute('target', '_blank')
    }

    return template.innerHTML
  } catch {
    return plainTextFallback(content)
  }
}
