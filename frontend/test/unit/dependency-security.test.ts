import { createRequire } from 'node:module'
import { describe, expect, it } from 'vitest'
import { makeRe, minimatch } from '../../vendor/minimatch-compat/index.js'

const require = createRequire(import.meta.url)
const swaggerRequire = createRequire(require.resolve('swagger-client'))
const merge = swaggerRequire('deepmerge')

describe('patched deepmerge dependency', () => {
  it('blocks prototype poisoning from either operand and null-prototype objects', () => {
    const poisoned = JSON.parse('{"__proto__":{"isAdmin":true},"constructor":{"prototype":{"isAdmin":true}},"safe":1}')
    for (const value of [merge(poisoned, {}), merge({}, poisoned), merge(Object.create(null), poisoned), merge({ nested: poisoned }, {})]) {
      expect(value.isAdmin).toBeUndefined()
      expect(value.nested?.isAdmin).toBeUndefined()
      expect(Object.getPrototypeOf(value)).toBe(Object.prototype)
    }
    expect(({} as { isAdmin?: boolean }).isAdmin).toBeUndefined()
  })

  it('preserves deep cloning, symbols, array and custom merge behavior', () => {
    const symbol = Symbol('value')
    const source = { nested: { a: 1 }, values: [1], [symbol]: { a: 1 } }
    const result = merge(source, { nested: { b: 2 }, values: [2], [symbol]: { b: 2 } })
    expect(result).toEqual({ nested: { a: 1, b: 2 }, values: [1, 2], [symbol]: { a: 1, b: 2 } })
    expect(result.nested).not.toBe(source.nested)
    expect(merge.all([{ a: 1 }, { b: 2 }])).toEqual({ a: 1, b: 2 })
    expect(merge({ values: [1] }, { values: [2] }, { arrayMerge: (_a: unknown, b: unknown) => b }).values).toEqual([2])
    expect(merge({ nested: {} }, { nested: { b: 2 } }, { customMerge: () => () => 'custom' }).nested).toBe('custom')
  })
})

describe('minimatch compatibility adapter', () => {
  it('preserves import-x allow/forbid predicates through patched matching', () => {
    expect(makeRe('lib/**').test('lib/internal/file')).toBe(true)
    expect(makeRe('lib/**').test('other/internal')).toBe(false)
    expect(minimatch('file.ts', '*.ts')).toBe(true)
    const cjs = require('../../vendor/minimatch-compat/index.cjs')
    expect(cjs.makeRe('lib/**').test('lib/internal/file')).toBe(true)
    expect(cjs.minimatch('file.js', '*.ts')).toBe(false)
  })
})
