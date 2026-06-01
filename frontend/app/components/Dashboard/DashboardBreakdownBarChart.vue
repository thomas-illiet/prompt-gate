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
  requests: number
  value: number
}

interface BreakdownTooltipParam {
  data?: BreakdownChartPoint
  name?: string
}

const option = computed<ECOption>(() => ({
  backgroundColor: 'transparent',
  animationDelay: (index: number) => index * 70,
  animationDuration: 650,
  animationEasing: 'cubicOut',
  tooltip: {
    ...dashboardTooltipOptions,
    trigger: 'axis',
    axisPointer: {
      type: 'shadow',
    },
    formatter: (params: unknown) => {
      const point = Array.isArray(params) ? params[0] : params
      const item = point as BreakdownTooltipParam
      const data = item.data

      return formatDashboardTooltip({
        estimatedCost: data?.estimatedCost,
        metrics: [
          { label: 'Tokens', value: `${formatNumber(data?.value)} tokens` },
          {
            label: 'Requests',
            value: `${formatNumber(data?.requests)} requests`,
          },
        ],
        title: item.name,
      })
    },
  },
  grid: {
    top: 10,
    left: '2%',
    right: '8%',
    bottom: '3%',
    containLabel: true,
  },
  xAxis: {
    type: 'value',
    axisLabel: {
      hideOverlap: true,
    },
  },
  yAxis: {
    type: 'category',
    inverse: true,
    data: props.items.map((item) => item.name),
  },
  series: [
    {
      name: 'Tokens',
      type: 'bar',
      barMaxWidth: 28,
      itemStyle: {
        borderRadius: [0, 6, 6, 0],
      },
      data: props.items.map((item) => ({
        estimatedCost: item.estimatedCost,
        requests: item.requests,
        value: item.totalTokens,
      })),
    },
  ],
}))
</script>

<template>
  <v-chart :option="option" autoresize />
</template>
