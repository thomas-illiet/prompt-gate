import type { MinimatchOptions } from 'minimatch-engine'
export * from 'minimatch-engine'

export function makeRe(pattern: string, options?: MinimatchOptions): { test(path: string): boolean }
