import { tokenize, type ShjToken } from 'rangi'
import type { SetupGuideSyntaxLanguage } from '~/types/setup-guides'

export interface SyntaxHighlightToken {
  text: string
  type?: ShjToken
}

export type SyntaxHighlightLine = SyntaxHighlightToken[]

const RANGI_LANGUAGE_BY_SETUP_GUIDE_LANGUAGE: Record<
  SetupGuideSyntaxLanguage,
  string
> = {
  plaintext: 'plain',
  bash: 'bash',
  python: 'py',
  go: 'go',
  java: 'java',
  csharp: 'cs',
  powershell: 'ps1',
  lua: 'lua',
  yaml: 'yaml',
  json: 'json',
  json5: 'json5',
}

const SYNTAX_LANGUAGE_LABELS: Record<SetupGuideSyntaxLanguage, string> = {
  plaintext: 'Plain text',
  bash: 'Bash',
  python: 'Python',
  go: 'Go',
  java: 'Java',
  csharp: 'C#',
  powershell: 'PowerShell',
  lua: 'Lua',
  yaml: 'YAML',
  json: 'JSON',
  json5: 'JSON5',
}

function isSetupGuideSyntaxLanguage(
  value: string | null | undefined,
): value is SetupGuideSyntaxLanguage {
  return Boolean(
    value &&
    Object.prototype.hasOwnProperty.call(
      RANGI_LANGUAGE_BY_SETUP_GUIDE_LANGUAGE,
      value,
    ),
  )
}

// syntaxLanguageLabel returns a stable display label with a plaintext fallback.
export function syntaxLanguageLabel(language?: string | null) {
  return isSetupGuideSyntaxLanguage(language)
    ? SYNTAX_LANGUAGE_LABELS[language]
    : SYNTAX_LANGUAGE_LABELS.plaintext
}

// tokenizeSyntaxCode highlights the complete source before splitting it by line.
export function tokenizeSyntaxCode(
  code: string,
  language?: string | null,
): SyntaxHighlightLine[] {
  const highlighted: SyntaxHighlightToken[] =
    isSetupGuideSyntaxLanguage(language) && language !== 'plaintext'
      ? tokenize(code, {
          lang: RANGI_LANGUAGE_BY_SETUP_GUIDE_LANGUAGE[language],
        })
      : [{ text: code }]
  const lines: SyntaxHighlightLine[] = [[]]

  for (const token of highlighted) {
    const parts = token.text.split('\n')
    parts.forEach((part, index) => {
      if (index > 0) lines.push([])
      if (part) lines.at(-1)!.push({ text: part, type: token.type })
    })
  }

  return lines
}
