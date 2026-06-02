<script setup lang="ts">
import { curveBumpX, curveBumpY, line } from 'd3'

import type {
  AdminPromptHistoryItem,
  PromptHistoryItem,
} from '~/types/user-service'
import { formatDurationMs, formatNumber } from '~/utils/formatters'

type PromptHistoryGraphItem = PromptHistoryItem | AdminPromptHistoryItem

const props = defineProps<{
  actorLabel: string
  prompt: PromptHistoryGraphItem
}>()

interface GraphNode {
  column: number
  key: string
  row: number
  title: string
  subtitle: string
  x: number
  y: number
}

interface GraphNodeSeed {
  key: string
  title: string
  subtitle: string
}

interface GraphEdge {
  key: string
  label: string
  labelWidth: number
  labelX: number
  labelY: number
  path: string
}

interface GraphEdgeSeed {
  key: string
  label: string
  labelWidth: number
}

const graphWidth = 1100
const graphMarginX = 48
const graphMarginY = 42
const graphBottomMetricSpace = 70
const maxColumns = 3
const nodeWidth = 160
const nodeHeight = 78
const rowGap = 118
const edgeLabelHeight = 28
const arrowInset = 18
const graphSvg = useTemplateRef<SVGSVGElement>('graphSvg')
const draggedNodeKey = shallowRef<string | null>(null)
const dragOffset = reactive({ x: 0, y: 0 })
const nodePositions = shallowRef<Record<string, { x: number; y: number }>>({})
const d3HorizontalLine = line<[number, number]>()
  .x((point) => point[0])
  .y((point) => point[1])
  .curve(curveBumpX)
const d3VerticalLine = line<[number, number]>()
  .x((point) => point[0])
  .y((point) => point[1])
  .curve(curveBumpY)

const markerId = computed(
  () =>
    `prompt-request-arrow-${props.prompt.id.replace(/[^A-Za-z0-9_-]/g, '-')}`,
)

const requestEdgeLabel = computed(() => {
  const ip = clientIpLabel(props.prompt)
  return ip ? `IP ${ip}` : 'request'
})

const nodeSeeds = computed<GraphNodeSeed[]>(() => [
  {
    key: 'actor',
    title: truncateLabel(props.actorLabel || 'You', 18),
    subtitle: 'Requester',
  },
  {
    key: 'gateway',
    title: 'PromptGate',
    subtitle: 'Gateway',
  },
  {
    key: 'provider',
    title: truncateLabel(props.prompt.provider, 18),
    subtitle: providerTypeLabel(props.prompt.providerType),
  },
  {
    key: 'model',
    title: truncateLabel(props.prompt.model, 20),
    subtitle: 'Model',
  },
  {
    key: 'response',
    title: 'Response',
    subtitle: formatDurationMs(props.prompt.durationMs),
  },
])

const edgeSeeds = computed<GraphEdgeSeed[]>(() => [
  {
    key: 'request',
    label: requestEdgeLabel.value,
    labelWidth: edgeLabelWidth(requestEdgeLabel.value, 88),
  },
  {
    key: 'input',
    label: `${formatNumber(props.prompt.inputTokens)} input`,
    labelWidth: edgeLabelWidth(
      `${formatNumber(props.prompt.inputTokens)} input`,
      118,
    ),
  },
  {
    key: 'route',
    label: 'route',
    labelWidth: 76,
  },
  {
    key: 'output',
    label: `${formatNumber(props.prompt.outputTokens)} output`,
    labelWidth: edgeLabelWidth(
      `${formatNumber(props.prompt.outputTokens)} output`,
      126,
    ),
  },
])

const columnCount = computed(() =>
  Math.min(maxColumns, Math.max(nodeSeeds.value.length, 1)),
)

const rowCount = computed(() =>
  Math.ceil(nodeSeeds.value.length / columnCount.value),
)

const graphHeight = computed(
  () =>
    graphMarginY +
    rowCount.value * nodeHeight +
    Math.max(rowCount.value - 1, 0) * rowGap +
    graphBottomMetricSpace,
)

const nodes = computed<GraphNode[]>(() =>
  nodeSeeds.value.map((node, index) => {
    const row = Math.floor(index / columnCount.value)
    const column = index % columnCount.value
    const rowStartIndex = row * columnCount.value
    const nodesInRow = Math.min(
      columnCount.value,
      nodeSeeds.value.length - rowStartIndex,
    )
    const fallback = {
      x: rowNodeX(column, nodesInRow),
      y: graphMarginY + nodeHeight / 2 + row * (nodeHeight + rowGap),
    }
    const position = nodePositions.value[node.key] ?? fallback

    return {
      ...node,
      column,
      row,
      x: position.x,
      y: position.y,
    }
  }),
)

const edges = computed<GraphEdge[]>(() =>
  edgeSeeds.value.map((edge, index) => {
    const source = nodes.value[index]
    const target = nodes.value[index + 1]
    if (!source || !target) {
      return {
        ...edge,
        labelX: graphWidth / 2,
        labelY: graphMarginY,
        path: '',
      }
    }

    return {
      ...edge,
      ...edgeLayout(edge, source, target),
    }
  }),
)

const durationLabel = computed(
  () => `duration: ${formatDurationMs(props.prompt.durationMs)}`,
)

const durationLabelY = computed(() => graphHeight.value - 28)

function nodeLeft(node: GraphNode) {
  return node.x - nodeWidth / 2
}

function nodeTop(node: GraphNode) {
  return node.y - nodeHeight / 2
}

function edgeLabelLeft(edge: GraphEdge) {
  return clamp(
    edge.labelX - edge.labelWidth / 2,
    8,
    graphWidth - edge.labelWidth - 8,
  )
}

function edgeLabelWidth(label: string, minWidth: number) {
  return Math.max(minWidth, label.length * 7 + 28)
}

function rowNodeX(column: number, nodesInRow: number) {
  const left = graphMarginX + nodeWidth / 2
  const right = graphWidth - graphMarginX - nodeWidth / 2
  if (nodesInRow <= 1) {
    return graphWidth / 2
  }

  const columnsToFill =
    nodesInRow < columnCount.value ? nodesInRow - 1 : columnCount.value - 1
  return left + (column * (right - left)) / columnsToFill
}

function edgeLayout(edge: GraphEdgeSeed, source: GraphNode, target: GraphNode) {
  if (source.row === target.row) {
    const direction = source.x <= target.x ? 1 : -1
    const startX = source.x + direction * (nodeWidth / 2 + arrowInset)
    const endX = target.x - direction * (nodeWidth / 2 + arrowInset)
    const labelX = (startX + endX) / 2
    const labelY = source.y - edgeLabelHeight / 2
    const path =
      d3HorizontalLine([
        [startX, source.y],
        [endX, target.y],
      ]) ?? ''

    return { labelX, labelY, path }
  }

  const startY = source.y + nodeHeight / 2 + arrowInset
  const endY = target.y - nodeHeight / 2 - arrowInset
  const turnY = (startY + endY) / 2
  const labelX = (source.x + target.x) / 2
  const labelY = turnY - edgeLabelHeight / 2
  const path =
    d3VerticalLine([
      [source.x, startY],
      [source.x, turnY],
      [target.x, turnY],
      [target.x, endY],
    ]) ?? ''

  return { labelX, labelY, path }
}

function clientIpLabel(prompt: PromptHistoryGraphItem) {
  if (!('clientIp' in prompt)) {
    return ''
  }

  return prompt.clientIp.trim() || 'unknown'
}

function pointerPoint(event: PointerEvent) {
  const svg = graphSvg.value
  if (!svg) {
    return null
  }

  const point = svg.createSVGPoint()
  const matrix = svg.getScreenCTM()
  if (!matrix) {
    return null
  }

  point.x = event.clientX
  point.y = event.clientY

  return point.matrixTransform(matrix.inverse())
}

function startNodeDrag(node: GraphNode, event: PointerEvent) {
  if (event.button !== 0) {
    return
  }

  const point = pointerPoint(event)
  if (!point) {
    return
  }

  draggedNodeKey.value = node.key
  dragOffset.x = point.x - node.x
  dragOffset.y = point.y - node.y
  const target = event.currentTarget as SVGElement
  target.setPointerCapture?.(event.pointerId)
  event.preventDefault()
}

function moveDraggedNode(event: PointerEvent) {
  const key = draggedNodeKey.value
  if (!key) {
    return
  }

  const point = pointerPoint(event)
  if (!point) {
    return
  }

  nodePositions.value = {
    ...nodePositions.value,
    [key]: {
      x: clamp(
        point.x - dragOffset.x,
        graphMarginX + nodeWidth / 2,
        graphWidth - graphMarginX - nodeWidth / 2,
      ),
      y: clamp(
        point.y - dragOffset.y,
        graphMarginY + nodeHeight / 2,
        graphHeight.value - graphBottomMetricSpace - nodeHeight / 2,
      ),
    },
  }
}

function stopNodeDrag() {
  draggedNodeKey.value = null
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max)
}

watch(
  () => props.prompt.id,
  () => {
    nodePositions.value = {}
    draggedNodeKey.value = null
  },
)

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
</script>

<template>
  <div class="prompt-history-request-graph">
    <svg
      ref="graphSvg"
      class="prompt-history-request-graph__svg"
      role="img"
      aria-label="Prompt request graph"
      :viewBox="`0 0 ${graphWidth} ${graphHeight}`"
      preserveAspectRatio="xMidYMid meet"
      @pointermove="moveDraggedNode"
      @pointerup="stopNodeDrag"
      @pointercancel="stopNodeDrag"
      @pointerleave="stopNodeDrag"
    >
      <defs>
        <marker
          :id="markerId"
          markerWidth="9"
          markerHeight="9"
          refX="8"
          refY="4.5"
          orient="auto"
          markerUnits="userSpaceOnUse"
        >
          <path
            class="prompt-history-request-graph__arrow-head"
            d="M 0 0 L 9 4.5 L 0 9 z"
          />
        </marker>
      </defs>

      <g class="prompt-history-request-graph__edges">
        <g v-for="edge in edges" :key="edge.key">
          <path
            class="prompt-history-request-graph__edge"
            :d="edge.path"
            fill="none"
            :marker-end="`url(#${markerId})`"
            vector-effect="non-scaling-stroke"
          />
          <rect
            class="prompt-history-request-graph__edge-label-box"
            :x="edgeLabelLeft(edge)"
            :y="edge.labelY"
            :width="edge.labelWidth"
            :height="edgeLabelHeight"
            rx="14"
          />
          <text
            class="prompt-history-request-graph__edge-label"
            :x="edge.labelX"
            :y="edge.labelY + 19"
            text-anchor="middle"
          >
            {{ edge.label }}
          </text>
        </g>

        <rect
          class="prompt-history-request-graph__duration-box"
          :x="graphWidth / 2 - 108"
          :y="durationLabelY - 21"
          width="216"
          height="34"
          rx="17"
        />
        <text
          class="prompt-history-request-graph__duration-label"
          :x="graphWidth / 2"
          :y="durationLabelY + 1"
          text-anchor="middle"
        >
          {{ durationLabel }}
        </text>
      </g>

      <g
        v-for="node in nodes"
        :key="node.key"
        class="prompt-history-request-graph__node"
        :class="{
          'prompt-history-request-graph__node--dragging':
            draggedNodeKey === node.key,
        }"
        role="button"
        tabindex="0"
        :aria-label="`Move ${node.title}`"
        @pointerdown="startNodeDrag(node, $event)"
      >
        <title>{{ node.title }} - {{ node.subtitle }}</title>
        <rect
          class="prompt-history-request-graph__node-box"
          :x="nodeLeft(node)"
          :y="nodeTop(node)"
          :width="nodeWidth"
          :height="nodeHeight"
          rx="8"
        />
        <text
          class="prompt-history-request-graph__node-title"
          :x="node.x"
          :y="nodeTop(node) + 31"
          text-anchor="middle"
        >
          {{ node.title }}
        </text>
        <text
          class="prompt-history-request-graph__node-subtitle"
          :x="node.x"
          :y="nodeTop(node) + 55"
          text-anchor="middle"
        >
          {{ node.subtitle }}
        </text>
      </g>
    </svg>
  </div>
</template>

<style scoped>
.prompt-history-request-graph {
  box-sizing: border-box;
  width: 100%;
  overflow-x: hidden;
  padding: 12px;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  border-radius: 8px;
  background: rgb(var(--app-shell-surface));
}

.prompt-history-request-graph__svg {
  display: block;
  width: 100%;
  height: auto;
  margin-inline: auto;
  touch-action: none;
  user-select: none;
}

.prompt-history-request-graph__edge {
  fill: none;
  stroke: rgba(var(--v-theme-primary), 0.72);
  stroke-linecap: round;
  stroke-width: 3;
}

.prompt-history-request-graph__edge-label-box {
  fill: rgb(var(--app-shell-surface-strong));
  stroke: rgba(var(--v-theme-primary), 0.18);
  stroke-width: 1;
}

.prompt-history-request-graph__arrow-head {
  fill: rgb(var(--v-theme-primary));
}

.prompt-history-request-graph__edge-label,
.prompt-history-request-graph__duration-label {
  font-size: 13px;
  font-weight: 700;
}

.prompt-history-request-graph__edge-label {
  fill: rgb(var(--app-shell-text-primary));
}

.prompt-history-request-graph__duration-box {
  fill: rgba(var(--v-theme-primary), 0.1);
  stroke: rgba(var(--v-theme-primary), 0.26);
  stroke-width: 1;
}

.prompt-history-request-graph__duration-label {
  fill: rgb(var(--app-shell-text-primary));
}

.prompt-history-request-graph__node-box {
  fill: rgb(var(--app-shell-surface-strong));
  stroke: rgba(var(--app-shell-border), 0.75);
  stroke-width: 1.5;
  transition:
    fill 0.16s ease,
    stroke 0.16s ease;
}

.prompt-history-request-graph__node {
  cursor: grab;
}

.prompt-history-request-graph__node--dragging {
  cursor: grabbing;
}

.prompt-history-request-graph__node--dragging
  .prompt-history-request-graph__node-box {
  fill: rgba(var(--v-theme-primary), 0.08);
  stroke: rgba(var(--v-theme-primary), 0.7);
}

.prompt-history-request-graph__node-title {
  fill: rgb(var(--app-shell-text-primary));
  font-size: 16px;
  font-weight: 800;
}

.prompt-history-request-graph__node-subtitle {
  fill: rgb(var(--app-shell-text-secondary));
  font-size: 12px;
  font-weight: 700;
}

@media (max-width: 640px) {
  .prompt-history-request-graph {
    padding: 8px;
  }
}
</style>
