const engine = require("minimatch-engine")
module.exports = { ...engine, makeRe(pattern, options) {
  const matcher = new engine.Minimatch(pattern, options)
  return { test: path => matcher.match(path) }
} }
