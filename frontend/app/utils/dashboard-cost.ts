import type { EstimatedCost } from '~/types/user-service'
import { formatCurrencyUsd } from '~/utils/formatters'

export const dashboardTooltipOptions = {
  appendTo: 'body',
  confine: true,
  extraCssText: 'max-width: 240px; white-space: normal;',
  renderMode: 'html',
} as const

const tooltipHtmlEntities: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
}

export function escapeTooltipHtml(value: string | number | null | undefined) {
  return String(value ?? '').replace(
    /[&<>"']/g,
    (char) => tooltipHtmlEntities[char] ?? char,
  )
}

export function formatTooltipLines(
  lines: Array<string | number | null | undefined>,
) {
  return lines
    .filter((line) => line != null && line !== '')
    .map((line) => escapeTooltipHtml(line))
    .join('<br />')
}

export function formatEstimatedCostTooltipLines(
  estimatedCost: EstimatedCost | null | undefined,
) {
  if (!estimatedCost) {
    return []
  }

  return [
    `Estimated cost: ${formatCurrencyUsd(estimatedCost.totalUsd)}`,
    `Input: ${formatCurrencyUsd(estimatedCost.inputUsd)}`,
    `Output: ${formatCurrencyUsd(estimatedCost.outputUsd)}`,
    `Embedding: ${formatCurrencyUsd(estimatedCost.embeddingUsd)}`,
  ]
}
