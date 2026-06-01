<script setup lang="ts">
import type { EstimatedCost, UsageBreakdown } from '~/types/user-service'
import {
  dashboardTooltipOptions,
  formatDashboardTooltip,
} from '~/utils/dashboard-cost'
import { formatNumber } from '~/utils/formatters'

const props = defineProps<{
  items: UsageBreakdown[]
}>()

interface BreakdownChartPoint {
  estimatedCost?: EstimatedCost
  requests?: number
  value?: number
}

const option = computed<ECOption>(() => ({
  backgroundColor: 'transparent',
  animationDuration: 760,
  animationEasing: 'cubicOut',
  tooltip: {
    ...dashboardTooltipOptions,
    trigger: 'item',
    formatter: (params: unknown) => {
      const point = Array.isArray(params) ? params[0] : params
      const item = point as {
        data?: BreakdownChartPoint
        name?: string
      }
      const data = item.data ?? {}
      return formatDashboardTooltip({
        estimatedCost: data.estimatedCost,
        metrics: [
          { label: 'Tokens', value: `${formatNumber(data.value)} tokens` },
          { label: 'Requests', value: `${formatNumber(data.requests)} requests` },
        ],
        title: item.name,
      })
    },
  },
  legend: {
    type: 'scroll',
    bottom: 0,
    left: 'center',
  },
  series: [
    {
      name: 'Tokens',
      type: 'pie',
      radius: ['42%', '68%'],
      center: ['50%', '42%'],
      avoidLabelOverlap: true,
      padAngle: 2,
      itemStyle: {
        borderRadius: 5,
      },
      emphasis: {
        scaleSize: 6,
      },
      data: props.items.map((item) => ({
        estimatedCost: item.estimatedCost,
        name: item.name,
        value: item.totalTokens,
        requests: item.requests,
      })),
    },
  ],
}))
</script>

<template>
  <v-chart :option="option" autoresize />
</template>
