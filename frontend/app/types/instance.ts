export const instanceEnvironments = [
  'development',
  'staging',
  'production',
] as const

export type InstanceEnvironment = (typeof instanceEnvironments)[number]
export type InstanceEnvironmentState = InstanceEnvironment | 'unknown'

export interface InstanceResponse {
  environment: InstanceEnvironment
}

export function isInstanceEnvironment(
  value: unknown,
): value is InstanceEnvironment {
  return instanceEnvironments.includes(value as InstanceEnvironment)
}
