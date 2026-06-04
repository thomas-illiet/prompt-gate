<script setup lang="ts">
import {
  buildPromptHistoryGraphLayout,
  clamp,
  promptHistoryGraphConfig,
  promptHistoryGraphEdgeLabelLeft,
  promptHistoryGraphNodeLeft,
  promptHistoryGraphNodeTop,
  type PromptHistoryGraphEdge,
  type PromptHistoryGraphItem,
  type PromptHistoryGraphNode,
  type PromptHistoryGraphPosition,
} from '~/utils/prompt-history-graph'

const props = defineProps<{
  actorLabel: string
  prompt: PromptHistoryGraphItem
}>()

const {
  edgeLabelHeight,
  graphBottomMetricSpace,
  graphMarginX,
  graphMarginY,
  graphWidth,
  nodeHeight,
  nodeWidth,
} = promptHistoryGraphConfig
const graphSvg = useTemplateRef<SVGSVGElement>('graphSvg')
const draggedNodeKey = shallowRef<string | null>(null)
const dragOffset = reactive({ x: 0, y: 0 })
const nodePositions = shallowRef<Record<string, PromptHistoryGraphPosition>>({})

const graphLayout = computed(() =>
  buildPromptHistoryGraphLayout({
    actorLabel: props.actorLabel,
    nodePositions: nodePositions.value,
    prompt: props.prompt,
  }),
)
const markerId = computed(() => graphLayout.value.markerId)
const graphHeight = computed(() => graphLayout.value.graphHeight)
const nodes = computed(() => graphLayout.value.nodes)
const edges = computed(() => graphLayout.value.edges)
const durationLabel = computed(() => graphLayout.value.durationLabel)
const durationLabelY = computed(() => graphLayout.value.durationLabelY)

function nodeLeft(node: PromptHistoryGraphNode) {
  return promptHistoryGraphNodeLeft(node)
}

function nodeTop(node: PromptHistoryGraphNode) {
  return promptHistoryGraphNodeTop(node)
}

function edgeLabelLeft(edge: PromptHistoryGraphEdge) {
  return promptHistoryGraphEdgeLabelLeft(edge)
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

function startNodeDrag(node: PromptHistoryGraphNode, event: PointerEvent) {
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

watch(
  () => props.prompt.id,
  () => {
    nodePositions.value = {}
    draggedNodeKey.value = null
  },
)
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
