import type { InstanceEnvironmentState } from '~/types/instance'

export interface InstancePresentation {
  label: string
  accent: string
  surface: string
  text: string
}

const presentations: Record<InstanceEnvironmentState, InstancePresentation> = {
  development: {
    label: 'Development',
    accent: '#2878c8',
    surface: 'rgba(40, 120, 200, 0.14)',
    text: 'light-dark(#1763aa, #8fc8ff)',
  },
  staging: {
    label: 'Staging',
    accent: '#d88216',
    surface: 'rgba(216, 130, 22, 0.14)',
    text: 'light-dark(#8a4d00, #ffc66d)',
  },
  production: {
    label: 'Production',
    accent: '#d84b47',
    surface: 'rgba(216, 75, 71, 0.14)',
    text: 'light-dark(#a32623, #ffaaa7)',
  },
  unknown: {
    label: 'Unknown',
    accent: '#687580',
    surface: 'rgba(104, 117, 128, 0.14)',
    text: 'light-dark(#53606b, #c1c9cf)',
  },
}

export function instancePresentation(
  environment: InstanceEnvironmentState,
): InstancePresentation {
  return presentations[environment]
}
