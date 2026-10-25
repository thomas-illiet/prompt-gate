export type SetupGuideCompatibility = 'openai' | 'anthropic' | 'both'
export type SetupGuideModelMode = 'single' | 'all' | 'none'

export const SETUP_GUIDE_SYNTAX_LANGUAGES = [
  'plaintext',
  'bash',
  'python',
  'go',
  'java',
  'csharp',
  'powershell',
  'lua',
  'yaml',
  'json',
  'json5',
] as const

export type SetupGuideSyntaxLanguage =
  (typeof SETUP_GUIDE_SYNTAX_LANGUAGES)[number]

export const SETUP_GUIDE_SYNTAX_LANGUAGE_OPTIONS: ReadonlyArray<{
  title: string
  value: SetupGuideSyntaxLanguage
}> = [
  { title: 'Plain text', value: 'plaintext' },
  { title: 'Bash', value: 'bash' },
  { title: 'Python', value: 'python' },
  { title: 'Go', value: 'go' },
  { title: 'Java', value: 'java' },
  { title: 'C#', value: 'csharp' },
  { title: 'PowerShell', value: 'powershell' },
  { title: 'Lua', value: 'lua' },
  { title: 'YAML', value: 'yaml' },
  { title: 'JSON', value: 'json' },
  { title: 'JSON5', value: 'json5' },
]

export interface SetupGuide {
  id: string
  identifier: string
  title: string
  subtitle: string
  icon: string
  compatibility: SetupGuideCompatibility
  modelMode: SetupGuideModelMode
  syntaxLanguage: SetupGuideSyntaxLanguage
  filePaths: string[]
  template: string
  enabled: boolean
  position: number
  createdAt: string
  updatedAt: string
}

export type SetupGuidePayload = Omit<
  SetupGuide,
  'id' | 'createdAt' | 'updatedAt'
>
