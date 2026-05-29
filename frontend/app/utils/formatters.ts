// formatDateTime renders optional timestamps for the French locale.
export function formatDateTime(value: string | null | undefined) {
  if (!value) {
    return 'Never'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }

  return new Intl.DateTimeFormat('fr-FR', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

// formatNumber renders optional numbers for the French locale.
export function formatNumber(value: number | null | undefined) {
  if (value == null || Number.isNaN(value)) {
    return '0'
  }

  return new Intl.NumberFormat('fr-FR').format(value)
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
    return `${new Intl.NumberFormat('fr-FR', {
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
