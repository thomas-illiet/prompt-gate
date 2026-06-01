import type { EstimatedCost } from '~/types/user-service'
import { formatCurrencyUsd } from '~/utils/formatters'

export const dashboardTooltipOptions = {
  appendTo: 'body',
  confine: true,
  extraCssText: [
    'max-width: 320px',
    'white-space: normal',
    'border-radius: 10px',
    'box-shadow: 0 14px 34px rgba(15, 23, 42, 0.18)',
  ].join('; '),
  padding: 0,
  renderMode: 'html',
} as const

interface DashboardTooltipMetric {
  label: string
  value: string | number | null | undefined
}

interface DashboardTooltipOptions {
  estimatedCost?: EstimatedCost | null
  metrics: DashboardTooltipMetric[]
  title: string | number | null | undefined
}

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

function tooltipRow(label: string, value: string | number | null | undefined) {
  if (value == null || value === '') {
    return ''
  }

  return [
    '<div style="display:grid;grid-template-columns:minmax(0, 1fr) auto;gap:18px;align-items:baseline;">',
    `<span style="min-width:0;color:#64748b;">${escapeTooltipHtml(label)}</span>`,
    `<strong style="color:#0f172a;font-weight:700;text-align:right;white-space:nowrap;">${escapeTooltipHtml(value)}</strong>`,
    '</div>',
  ].join('')
}

function formatEstimatedCostRows(
  estimatedCost: EstimatedCost | null | undefined,
) {
  if (!estimatedCost) {
    return ''
  }

  const rows = [
    tooltipRow('Total', formatCurrencyUsd(estimatedCost.totalUsd)),
    tooltipRow('Input', formatCurrencyUsd(estimatedCost.inputUsd)),
    tooltipRow('Output', formatCurrencyUsd(estimatedCost.outputUsd)),
    tooltipRow('Embedding', formatCurrencyUsd(estimatedCost.embeddingUsd)),
  ].join('')

  return [
    '<div style="height:1px;background:rgba(15, 23, 42, 0.12);"></div>',
    '<div style="display:grid;gap:7px;">',
    '<div style="color:#475569;font-size:11px;font-weight:700;">Estimated cost</div>',
    rows,
    '</div>',
  ].join('')
}

export function formatDashboardTooltip(options: DashboardTooltipOptions) {
  const metricRows = options.metrics
    .map((metric) => tooltipRow(metric.label, metric.value))
    .join('')

  return [
    '<div data-dashboard-tooltip="true" style="display:grid;gap:10px;min-width:220px;max-width:320px;padding:12px 14px;color:#0f172a;font-size:12px;line-height:1.45;">',
    `<div style="min-width:0;padding-bottom:8px;border-bottom:1px solid rgba(15, 23, 42, 0.12);color:#111827;font-size:13px;font-weight:800;overflow-wrap:anywhere;">${escapeTooltipHtml(options.title)}</div>`,
    `<div style="display:grid;gap:7px;">${metricRows}</div>`,
    formatEstimatedCostRows(options.estimatedCost),
    '</div>',
  ].join('')
}
