import { Minimatch } from "minimatch-engine"
export * from "minimatch-engine"
// eslint-plugin-import-x only consumes makeRe().test(); no regex is constructed.
export const makeRe = (pattern, options) => {
  const matcher = new Minimatch(pattern, options)
  return { test: path => matcher.match(path) }
}
