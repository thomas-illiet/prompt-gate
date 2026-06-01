<script setup lang="ts">
import type { DailyUsage, EstimatedCost } from '~/types/user-service'
import {
  dashboardTooltipOptions,
  formatEstimatedCostTooltipLines,
  formatTooltipLines,
} from '~/utils/dashboard-cost'
import { formatNumber } from '~/utils/formatters'

const props = defineProps<{
  daily: DailyUsage[]
}>()

interface ActivityChartPoint {
  estimatedCost?: EstimatedCost
  value: number
}

interface ActivityTooltipParam {
  axisValue?: number | string
  axisValueLabel?: string
  data?: ActivityChartPoint
  seriesName?: string
  value?: number
}

function activityPoint(value: number, item: DailyUsage): ActivityChartPoint {
  return {
    estimatedCost: item.estimatedCost,
    value,
  }
}

const option = computed<ECOption>(() => ({
  backgroundColor: 'transparent',
  animationDuration: 700,
  animationEasing: 'cubicOut',
  tooltip: {
    ...dashboardTooltipOptions,
    trigger: 'axis',
    formatter: (params: unknown) => {
      const points = (Array.isArray(params) ? params : [params]).map(
        (point) => point as ActivityTooltipParam,
      )
      const firstPoint = points[0]
      const estimatedCost = points.find((point) => point.data?.estimatedCost)
        ?.data?.estimatedCost

      return formatTooltipLines([
        firstPoint?.axisValueLabel ?? firstPoint?.axisValue,
        ...points.map(
          (point) =>
            `${point.seriesName ?? 'Value'}: ${formatNumber(
              point.data?.value ?? point.value,
            )}`,
        ),
        ...formatEstimatedCostTooltipLines(estimatedCost),
      ])
    },
  },
  legend: {
    top: 0,
    data: ['Requests', 'Input tokens', 'Output tokens', 'Embedding tokens'],
  },
  grid: {
    top: 44,
    left: '2%',
    right: '2%',
    bottom: '3%',
    containLabel: true,
  },
  xAxis: {
    type: 'category',
    data: props.daily.map((item) => item.date),
  },
  yAxis: [
    {
      type: 'value',
      name: 'Requests',
    },
    {
      type: 'value',
      name: 'Tokens',
    },
  ],
  series: [
    {
      name: 'Requests',
      type: 'line',
      smooth: true,
      showSymbol: false,
      areaStyle: {
        opacity: 0.08,
      },
      emphasis: {
        focus: 'series',
      },
      data: props.daily.map((item) => activityPoint(item.requests, item)),
    },
    {
      name: 'Input tokens',
      type: 'bar',
      yAxisIndex: 1,
      stack: 'tokens',
      barMaxWidth: 24,
      itemStyle: {
        borderRadius: [0, 0, 4, 4],
      },
      emphasis: {
        focus: 'series',
      },
      data: props.daily.map((item) =>
        activityPoint(item.completionInputTokens, item),
      ),
    },
    {
      name: 'Output tokens',
      type: 'bar',
      yAxisIndex: 1,
      stack: 'tokens',
      barMaxWidth: 24,
      emphasis: {
        focus: 'series',
      },
      data: props.daily.map((item) =>
        activityPoint(item.completionOutputTokens, item),
      ),
    },
    {
      name: 'Embedding tokens',
      type: 'bar',
      yAxisIndex: 1,
      stack: 'tokens',
      barMaxWidth: 24,
      itemStyle: {
        borderRadius: [4, 4, 0, 0],
      },
      emphasis: {
        focus: 'series',
      },
      data: props.daily.map((item) =>
        activityPoint(item.embeddingTokens, item),
      ),
    },
  ],
}))
</script>

<template>
  <v-chart :option="option" autoresize />
</template>
