const APP_LOCALE = 'en-US'

// formatDateTime renders optional timestamps for the application locale.
export function formatDateTime(value: string | null | undefined) {
  if (!value) {
    return 'Never'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }

  return new Intl.DateTimeFormat(APP_LOCALE, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

// formatDate renders optional date values for the application locale.
export function formatDate(value: string | null | undefined) {
  if (!value) {
    return 'Unknown'
  }

  const date = /^\d{4}-\d{2}-\d{2}$/.test(value)
    ? new Date(`${value}T00:00:00Z`)
    : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }

  return new Intl.DateTimeFormat(APP_LOCALE, {
    dateStyle: 'medium',
    timeZone: 'UTC',
  }).format(date)
}

// formatNumber renders optional numbers for the application locale.
export function formatNumber(value: number | null | undefined) {
  if (value == null || Number.isNaN(value)) {
    return '0'
  }

  return new Intl.NumberFormat(APP_LOCALE).format(value)
}

// formatCompactNumber renders large values in a compact, scannable format.
export function formatCompactNumber(value: number | null | undefined) {
  if (value == null || Number.isNaN(value)) {
    return '0'
  }

  return new Intl.NumberFormat(APP_LOCALE, {
    maximumFractionDigits: value >= 1000 ? 1 : 0,
    notation: 'compact',
  }).format(value)
}

// formatCurrencyUsd renders optional USD amounts, including tiny usage estimates.
export function formatCurrencyUsd(value: number | null | undefined) {
  if (value == null || !Number.isFinite(value)) {
    return '$0.00'
  }

  const isTinyAmount = Math.abs(value) > 0 && Math.abs(value) < 0.01
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: isTinyAmount ? 6 : 2,
    maximumFractionDigits: isTinyAmount ? 6 : 2,
  }).format(value)
}

// formatDurationMs renders optional execution durations for table rows.
export function formatDurationMs(value: number | null | undefined) {
  if (value == null) {
    return 'Pending'
  }
  if (!Number.isFinite(value) || value < 0) {
    return 'Unknown'
  }
  if (value < 1000) {
    return `${Math.round(value)} ms`
  }
  if (value < 60000) {
    const seconds = value / 1000
    return `${new Intl.NumberFormat(APP_LOCALE, {
      maximumFractionDigits: 1,
    }).format(seconds)} s`
  }

  const totalSeconds = Math.round(value / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes < 60) {
    return seconds === 0 ? `${minutes} min` : `${minutes} min ${seconds} s`
  }

  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  return remainingMinutes === 0
    ? `${hours} h`
    : `${hours} h ${remainingMinutes} min`
}
