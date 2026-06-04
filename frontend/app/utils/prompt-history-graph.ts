import { curveBumpX, curveBumpY, line } from 'd3'

import type {
  AdminPromptHistoryItem,
  PromptHistoryItem,
} from '~/types/user-service'
import { formatDurationMs, formatNumber } from '~/utils/formatters'

export type PromptHistoryGraphItem = PromptHistoryItem | AdminPromptHistoryItem

export interface PromptHistoryGraphPosition {
  x: number
  y: number
}

export interface PromptHistoryGraphNode extends PromptHistoryGraphPosition {
  column: number
  key: string
  row: number
  title: string
  subtitle: string
}

export interface PromptHistoryGraphEdge {
  key: string
  label: string
  labelWidth: number
  labelX: number
  labelY: number
  path: string
}

interface PromptHistoryGraphNodeSeed {
  key: string
  title: string
  subtitle: string
}

interface PromptHistoryGraphEdgeSeed {
  key: string
  label: string
  labelWidth: number
}

export const promptHistoryGraphConfig = {
  arrowInset: 18,
  edgeLabelHeight: 28,
  graphBottomMetricSpace: 70,
  graphMarginX: 48,
  graphMarginY: 42,
  graphWidth: 1100,
  maxColumns: 3,
  nodeHeight: 78,
  nodeWidth: 160,
  rowGap: 118,
} as const

const d3HorizontalLine = line<[number, number]>()
  .x((point) => point[0])
  .y((point) => point[1])
  .curve(curveBumpX)

const d3VerticalLine = line<[number, number]>()
  .x((point) => point[0])
  .y((point) => point[1])
  .curve(curveBumpY)

export function buildPromptHistoryGraphLayout(options: {
  actorLabel: string
  nodePositions?: Record<string, PromptHistoryGraphPosition>
  prompt: PromptHistoryGraphItem
}) {
  const nodePositions = options.nodePositions ?? {}
  const nodeSeeds = promptHistoryGraphNodeSeeds(
    options.actorLabel,
    options.prompt,
  )
  const edgeSeeds = promptHistoryGraphEdgeSeeds(options.prompt)
  const columnCount = Math.min(
    promptHistoryGraphConfig.maxColumns,
    Math.max(nodeSeeds.length, 1),
  )
  const rowCount = Math.ceil(nodeSeeds.length / columnCount)
  const graphHeight =
    promptHistoryGraphConfig.graphMarginY +
    rowCount * promptHistoryGraphConfig.nodeHeight +
    Math.max(rowCount - 1, 0) * promptHistoryGraphConfig.rowGap +
    promptHistoryGraphConfig.graphBottomMetricSpace

  const nodes = nodeSeeds.map((node, index) => {
    const row = Math.floor(index / columnCount)
    const column = index % columnCount
    const rowStartIndex = row * columnCount
    const nodesInRow = Math.min(columnCount, nodeSeeds.length - rowStartIndex)
    const fallback = {
      x: rowNodeX(column, nodesInRow, columnCount),
      y:
        promptHistoryGraphConfig.graphMarginY +
        promptHistoryGraphConfig.nodeHeight / 2 +
        row *
          (promptHistoryGraphConfig.nodeHeight +
            promptHistoryGraphConfig.rowGap),
    }
    const position = nodePositions[node.key] ?? fallback

    return {
      ...node,
      column,
      row,
      x: position.x,
      y: position.y,
    }
  })

  const edges = edgeSeeds.map((edge, index) => {
    const source = nodes[index]
    const target = nodes[index + 1]
    if (!source || !target) {
      return {
        ...edge,
        labelX: promptHistoryGraphConfig.graphWidth / 2,
        labelY: promptHistoryGraphConfig.graphMarginY,
        path: '',
      }
    }

    return {
      ...edge,
      ...edgeLayout(edge, source, target),
    }
  })

  return {
    durationLabel: `duration: ${formatDurationMs(options.prompt.durationMs)}`,
    durationLabelY: graphHeight - 28,
    edges,
    graphHeight,
    markerId: `prompt-request-arrow-${options.prompt.id.replace(
      /[^A-Za-z0-9_-]/g,
      '-',
    )}`,
    nodes,
  }
}

export function promptHistoryGraphEdgeLabelLeft(edge: PromptHistoryGraphEdge) {
  return clamp(
    edge.labelX - edge.labelWidth / 2,
    8,
    promptHistoryGraphConfig.graphWidth - edge.labelWidth - 8,
  )
}

export function promptHistoryGraphNodeLeft(node: PromptHistoryGraphNode) {
  return node.x - promptHistoryGraphConfig.nodeWidth / 2
}

export function promptHistoryGraphNodeTop(node: PromptHistoryGraphNode) {
  return node.y - promptHistoryGraphConfig.nodeHeight / 2
}

export function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max)
}

function promptHistoryGraphNodeSeeds(
  actorLabel: string,
  prompt: PromptHistoryGraphItem,
): PromptHistoryGraphNodeSeed[] {
  return [
    {
      key: 'actor',
      title: truncateLabel(actorLabel || 'You', 18),
      subtitle: 'Requester',
    },
    {
      key: 'gateway',
      title: 'PromptGate',
      subtitle: 'Gateway',
    },
    {
      key: 'provider',
      title: truncateLabel(prompt.provider, 18),
      subtitle: providerTypeLabel(prompt.providerType),
    },
    {
      key: 'model',
      title: truncateLabel(prompt.model, 20),
      subtitle: 'Model',
    },
    {
      key: 'response',
      title: 'Response',
      subtitle: formatDurationMs(prompt.durationMs),
    },
  ]
}

function promptHistoryGraphEdgeSeeds(
  prompt: PromptHistoryGraphItem,
): PromptHistoryGraphEdgeSeed[] {
  const requestLabel = requestEdgeLabel(prompt)
  const inputLabel = `${formatNumber(prompt.inputTokens)} input`
  const outputLabel = `${formatNumber(prompt.outputTokens)} output`

  return [
    {
      key: 'request',
      label: requestLabel,
      labelWidth: edgeLabelWidth(requestLabel, 88),
    },
    {
      key: 'input',
      label: inputLabel,
      labelWidth: edgeLabelWidth(inputLabel, 118),
    },
    {
      key: 'route',
      label: 'route',
      labelWidth: 76,
    },
    {
      key: 'output',
      label: outputLabel,
      labelWidth: edgeLabelWidth(outputLabel, 126),
    },
  ]
}

function requestEdgeLabel(prompt: PromptHistoryGraphItem) {
  const ip = clientIpLabel(prompt)
  return ip ? `IP ${ip}` : 'request'
}

function clientIpLabel(prompt: PromptHistoryGraphItem) {
  if (!('clientIp' in prompt)) {
    return ''
  }

  return prompt.clientIp.trim() || 'unknown'
}

function edgeLabelWidth(label: string, minWidth: number) {
  return Math.max(minWidth, label.length * 7 + 28)
}

function rowNodeX(column: number, nodesInRow: number, columnCount: number) {
  const left =
    promptHistoryGraphConfig.graphMarginX +
    promptHistoryGraphConfig.nodeWidth / 2
  const right =
    promptHistoryGraphConfig.graphWidth -
    promptHistoryGraphConfig.graphMarginX -
    promptHistoryGraphConfig.nodeWidth / 2
  if (nodesInRow <= 1) {
    return promptHistoryGraphConfig.graphWidth / 2
  }

  const columnsToFill =
    nodesInRow < columnCount ? nodesInRow - 1 : columnCount - 1
  return left + (column * (right - left)) / columnsToFill
}

function edgeLayout(
  edge: PromptHistoryGraphEdgeSeed,
  source: PromptHistoryGraphNode,
  target: PromptHistoryGraphNode,
) {
  if (source.row === target.row) {
    const direction = source.x <= target.x ? 1 : -1
    const startX =
      source.x +
      direction *
        (promptHistoryGraphConfig.nodeWidth / 2 +
          promptHistoryGraphConfig.arrowInset)
    const endX =
      target.x -
      direction *
        (promptHistoryGraphConfig.nodeWidth / 2 +
          promptHistoryGraphConfig.arrowInset)
    const labelX = (startX + endX) / 2
    const labelY = source.y - promptHistoryGraphConfig.edgeLabelHeight / 2
    const path =
      d3HorizontalLine([
        [startX, source.y],
        [endX, target.y],
      ]) ?? ''

    return { labelX, labelY, path }
  }

  const startY =
    source.y +
    promptHistoryGraphConfig.nodeHeight / 2 +
    promptHistoryGraphConfig.arrowInset
  const endY =
    target.y -
    promptHistoryGraphConfig.nodeHeight / 2 -
    promptHistoryGraphConfig.arrowInset
  const turnY = (startY + endY) / 2
  const labelX = (source.x + target.x) / 2
  const labelY = turnY - promptHistoryGraphConfig.edgeLabelHeight / 2
  const path =
    d3VerticalLine([
      [source.x, startY],
      [source.x, turnY],
      [target.x, turnY],
      [target.x, endY],
    ]) ?? ''

  return { labelX, labelY, path }
}

function truncateLabel(value: string, maxLength: number) {
  if (value.length <= maxLength) {
    return value
  }

  return `${value.slice(0, Math.max(maxLength - 1, 1))}...`
}

function providerTypeLabel(providerType: string) {
  if (!providerType) {
    return 'Provider'
  }

  return providerType.charAt(0).toUpperCase() + providerType.slice(1)
}
