export type SetupGuideCompatibility = 'openai' | 'anthropic' | 'both'
export type SetupGuideModelMode = 'single' | 'all' | 'none'

export interface SetupGuide {
  id: string
  identifier: string
  title: string
  subtitle: string
  icon: string
  compatibility: SetupGuideCompatibility
  modelMode: SetupGuideModelMode
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
