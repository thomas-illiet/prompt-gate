<script setup lang="ts">
import type { UsageBreakdown } from '~/types/user-service'
import { formatNumber } from '~/utils/formatters'

const props = defineProps<{
  items: UsageBreakdown[]
}>()

const option = computed<ECOption>(() => ({
  backgroundColor: 'transparent',
  animationDuration: 760,
  animationEasing: 'cubicOut',
  tooltip: {
    trigger: 'item',
    formatter: (params: unknown) => {
      const point = Array.isArray(params) ? params[0] : params
      const item = point as {
        data?: { requests?: number; value?: number }
        name?: string
      }
      const data = item.data ?? {}
      return [
        item.name,
        `${formatNumber(data.value)} tokens`,
        `${formatNumber(data.requests)} requests`,
      ].join('<br />')
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
