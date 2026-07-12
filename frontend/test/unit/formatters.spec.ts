import { describe, expect, it } from 'vitest'

import { formatCurrencyUsd, formatDurationMs } from '../../app/utils/formatters'

describe('formatDurationMs', () => {
  it('formats milliseconds, seconds, minutes, and missing durations', () => {
    expect(formatDurationMs(250)).toBe('250 ms')
    expect(formatDurationMs(1250)).toBe('1.3 s')
    expect(formatDurationMs(90000)).toBe('1 min 30 s')
    expect(formatDurationMs(null)).toBe('Pending')
  })

  it('handles invalid durations', () => {
    expect(formatDurationMs(Number.NaN)).toBe('Unknown')
    expect(formatDurationMs(-1)).toBe('Unknown')
  })
})

describe('formatCurrencyUsd', () => {
  it('formats normal and tiny USD values', () => {
    expect(formatCurrencyUsd(12.3)).toBe('$12.30')
    expect(formatCurrencyUsd(0.00022034)).toBe('$0.000220')
    expect(formatCurrencyUsd(null)).toBe('$0.00')
  })
})
