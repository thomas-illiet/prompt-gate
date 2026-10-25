export const promptGateLightThemeColors = {
  background: '#F9F6ED',
  error: '#F06F57',
  info: '#60A4CD',
  'on-background': '#143428',
  'on-error': '#143428',
  'on-info': '#143428',
  'on-primary': '#FFFFFF',
  'on-secondary': '#FFFFFF',
  'on-success': '#143428',
  'on-surface': '#143428',
  'on-surface-variant': '#364C43',
  'on-warning': '#143428',
  primary: '#008553',
  secondary: '#143428',
  success: '#2AAE77',
  surface: '#FFFCF4',
  'surface-variant': '#EFF5E9',
  warning: '#E5AA3D',
} as const

export const promptGateChartColors = [
  promptGateLightThemeColors.primary,
  promptGateLightThemeColors.warning,
  promptGateLightThemeColors.error,
  promptGateLightThemeColors.info,
  promptGateLightThemeColors.success,
  promptGateLightThemeColors.secondary,
] as const
