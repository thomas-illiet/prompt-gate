<script setup lang="ts">
import type { UsageBreakdown } from '~/types/user-service'

const props = defineProps<{
  items: UsageBreakdown[]
}>()

const option = computed<ECOption>(() => ({
  backgroundColor: 'transparent',
  animationDelay: (index: number) => index * 70,
  animationDuration: 650,
  animationEasing: 'cubicOut',
  tooltip: {
    trigger: 'axis',
    axisPointer: {
      type: 'shadow',
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
      data: props.items.map((item) => item.totalTokens),
    },
  ],
}))
</script>

<template>
  <v-chart :option="option" autoresize />
</template>
