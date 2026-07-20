<script setup lang="ts">
import type { DailyUsage, EstimatedCost } from '~/types/user-service'
import {
  dashboardTooltipOptions,
  formatDashboardTooltip,
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

      return formatDashboardTooltip({
        estimatedCost,
        metrics: points.map((point) => ({
          label: point.seriesName ?? 'Value',
          value: formatNumber(point.data?.value ?? point.value),
        })),
        title: firstPoint?.axisValueLabel ?? firstPoint?.axisValue,
      })
    },
  },
  legend: {
    top: 0,
    left: 'center',
    itemGap: 8,
    itemWidth: 14,
    itemHeight: 8,
    formatter: (name: string) => name.replace(' tokens', ''),
    textStyle: {
      fontSize: 10,
    },
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
  <div class="dashboard-usage-line-chart">
    <v-chart
      aria-label="Daily usage chart. Detailed daily values follow."
      autoresize
      class="dashboard-usage-line-chart__visual"
      :option="option"
      role="img"
    />

    <table class="dashboard-usage-line-chart__screen-reader-table">
      <caption>
        Daily usage by date
      </caption>
      <thead>
        <tr>
          <th scope="col">Date</th>
          <th scope="col">Requests</th>
          <th scope="col">Input tokens</th>
          <th scope="col">Output tokens</th>
          <th scope="col">Embedding tokens</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in props.daily" :key="item.date">
          <th scope="row">{{ item.date }}</th>
          <td>{{ item.requests }}</td>
          <td>{{ item.completionInputTokens }}</td>
          <td>{{ item.completionOutputTokens }}</td>
          <td>{{ item.embeddingTokens }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.dashboard-usage-line-chart {
  position: relative;
}

.dashboard-usage-line-chart__visual {
  width: 100%;
  height: 100%;
}

.dashboard-usage-line-chart__screen-reader-table {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  clip-path: inset(50%);
  white-space: nowrap;
  border: 0;
}
</style>
