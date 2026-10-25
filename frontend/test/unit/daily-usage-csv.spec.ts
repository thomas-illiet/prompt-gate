import { describe, expect, it } from 'vitest'

import type { DailyUsage } from '../../app/types/user-service'
import { dailyUsageToCsv } from '../../app/utils/daily-usage-csv'

const day: DailyUsage = {
  date: '2026-10-24',
  requests: 4,
  inputTokens: 800,
  outputTokens: 200,
  completionInputTokens: 800,
  completionOutputTokens: 200,
  completionTokens: 1_000,
  embeddingTokens: 0,
  totalTokens: 1_000,
  estimatedCost: {
    inputUsd: 0.8,
    outputUsd: 0.4,
    embeddingUsd: 0,
    totalUsd: 1.2,
    rates: {
      inputUsdPer1MTokens: 1,
      outputUsdPer1MTokens: 2,
      embeddingUsdPer1MTokens: 0.1,
    },
  },
}

describe('dailyUsageToCsv', () => {
  it.each([
    ['requests', '"Date","API requests"\r\n"2026-10-24","4"'],
    ['tokens', '"Date","Tokens"\r\n"2026-10-24","1000"'],
    ['cost', '"Date","Estimated cost (USD)"\r\n"2026-10-24","1.2"'],
  ] as const)('exports the %s daily view with raw values', (view, expected) => {
    expect(dailyUsageToCsv([day], view)).toBe(expected)
  })

  it('exports all columns displayed by annual activity', () => {
    expect(dailyUsageToCsv([day], 'annual')).toBe(
      '"Date","API requests","Tokens"\r\n"2026-10-24","4","1000"',
    )
  })

  it('keeps unavailable estimated cost cells empty', () => {
    expect(
      dailyUsageToCsv([{ ...day, estimatedCost: undefined }], 'cost'),
    ).toBe('"Date","Estimated cost (USD)"\r\n"2026-10-24",""')
  })
})
