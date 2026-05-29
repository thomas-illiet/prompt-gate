<script setup lang="ts">
import type { DailyUsage } from '~/types/user-service'

const props = defineProps<{
  daily: DailyUsage[]
}>()

const option = computed<ECOption>(() => ({
  backgroundColor: 'transparent',
  animationDuration: 700,
  animationEasing: 'cubicOut',
  tooltip: {
    trigger: 'axis',
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
      data: props.daily.map((item) => item.requests),
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
      data: props.daily.map((item) => item.completionInputTokens),
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
      data: props.daily.map((item) => item.completionOutputTokens),
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
      data: props.daily.map((item) => item.embeddingTokens),
    },
  ],
}))
</script>

<template>
  <v-chart :option="option" autoresize />
</template>
