import { describe, expect, it } from 'vitest'

import {
  syntaxLanguageLabel,
  tokenizeSyntaxCode,
} from '../../app/utils/syntax-highlighting'

const sourceFromLines = (
  lines: ReturnType<typeof tokenizeSyntaxCode>,
): string =>
  lines.map((line) => line.map((token) => token.text).join('')).join('\n')

describe('syntax highlighting', () => {
  it.each([
    ['json', '{"model":"gpt-4.1","stream":true}'],
    ['bash', 'curl https://example.com -H "Authorization: Bearer token"'],
    ['python', 'from openai import OpenAI\nclient = OpenAI()'],
  ])('tokenizes %s while preserving the source', (language, source) => {
    const lines = tokenizeSyntaxCode(source, language)

    expect(sourceFromLines(lines)).toBe(source)
    expect(lines.flat().some((token) => token.type)).toBe(true)
  })

  it('tokenizes multiline constructs before splitting them into lines', () => {
    const source = '/* first line\nsecond line */\nclass Client {}'
    const lines = tokenizeSyntaxCode(source, 'java')

    expect(sourceFromLines(lines)).toBe(source)
    expect(lines[0]?.some((token) => token.type === 'cmnt')).toBe(true)
    expect(lines[1]?.some((token) => token.type === 'cmnt')).toBe(true)
  })

  it('falls back to escaped plaintext for unknown languages', () => {
    const source = '<script>alert("unsafe")</script>'
    const lines = tokenizeSyntaxCode(source, 'unknown-language')

    expect(sourceFromLines(lines)).toBe(source)
    expect(lines.flat().every((token) => token.type === undefined)).toBe(true)
    expect(syntaxLanguageLabel('unknown-language')).toBe('Plain text')
  })

  it('preserves empty and trailing lines', () => {
    const source = 'first\n\nlast\n'
    const lines = tokenizeSyntaxCode(source, 'plaintext')

    expect(lines).toHaveLength(4)
    expect(sourceFromLines(lines)).toBe(source)
  })
})
