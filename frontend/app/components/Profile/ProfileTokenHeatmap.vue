<script setup lang="ts">
import type { ProfileTokenUsageDay } from '~/composables/useProfileTokenUsage'
import { formatDate, formatNumber } from '~/utils/formatters'

const props = defineProps<{
  days: ProfileTokenUsageDay[]
}>()

interface HeatmapCell {
  day: ProfileTokenUsageDay | null
  key: string
}

interface HeatmapWeek {
  cells: HeatmapCell[]
  key: string
  monthLabel: string
}

const DATE_KEY_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/
const MONTH_FORMATTER = new Intl.DateTimeFormat('en-US', {
  month: 'short',
  timeZone: 'UTC',
})
const WEEKDAY_LABELS = ['Mon', '', 'Wed', '', 'Fri', '', 'Sun']

const maxTokens = computed(() =>
  props.days.reduce((max, day) => Math.max(max, day.totalTokens), 0),
)

const weeks = computed<HeatmapWeek[]>(() => {
  if (props.days.length === 0) {
    return []
  }

  const leadingCells = mondayFirstWeekdayIndex(props.days[0]?.date)
  const cells: HeatmapCell[] = [
    ...Array.from({ length: leadingCells }, (_, index) => ({
      day: null,
      key: `leading-${index}`,
    })),
    ...props.days.map((day) => ({
      day,
      key: day.date,
    })),
  ]
  const trailingCells = (7 - (cells.length % 7)) % 7
  cells.push(
    ...Array.from({ length: trailingCells }, (_, index) => ({
      day: null,
      key: `trailing-${index}`,
    })),
  )

  return Array.from({ length: cells.length / 7 }, (_, weekIndex) => {
    const weekCells = cells.slice(weekIndex * 7, weekIndex * 7 + 7)

    return {
      cells: weekCells,
      key: `week-${weekIndex}`,
      monthLabel: monthLabelForWeek(weekCells, weekIndex),
    }
  })
})

const gridTemplateColumns = computed(
  () => `repeat(${weeks.value.length}, var(--profile-token-cell-size))`,
)

function activityLevel(day: ProfileTokenUsageDay) {
  if (day.totalTokens <= 0 || maxTokens.value <= 0) {
    return 0
  }

  const ratio = day.totalTokens / maxTokens.value
  if (ratio <= 0.25) {
    return 1
  }
  if (ratio <= 0.5) {
    return 2
  }
  if (ratio <= 0.75) {
    return 3
  }
  return 4
}

function cellLabel(day: ProfileTokenUsageDay) {
  return `${formatDate(day.date)}: ${formatNumber(
    day.totalTokens,
  )} tokens, ${formatNumber(day.requests)} requests`
}

function monthLabelForWeek(cells: HeatmapCell[], weekIndex: number) {
  const labelDay =
    cells.find((cell) => {
      if (!cell.day) {
        return false
      }
      return weekIndex === 0 || cell.day.date.endsWith('-01')
    })?.day ?? null

  if (!labelDay) {
    return ''
  }

  return MONTH_FORMATTER.format(parseDateKey(labelDay.date))
}

function mondayFirstWeekdayIndex(dateKey: string | undefined) {
  const date = parseDateKey(dateKey)
  if (Number.isNaN(date.getTime())) {
    return 0
  }

  return (date.getUTCDay() + 6) % 7
}

function parseDateKey(value: string | undefined) {
  const match = value ? DATE_KEY_PATTERN.exec(value) : null
  if (!match) {
    return new Date(Number.NaN)
  }

  return new Date(
    Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])),
  )
}
</script>

<template>
  <div class="profile-token-heatmap" data-test="token-heatmap">
    <div class="profile-token-heatmap__scroll">
      <div class="profile-token-heatmap__grid">
        <div class="profile-token-heatmap__month-spacer" aria-hidden="true" />
        <div
          class="profile-token-heatmap__months"
          :style="{ gridTemplateColumns }"
          aria-hidden="true"
        >
          <span
            v-for="week in weeks"
            :key="`${week.key}-month`"
            data-test="token-heatmap-month"
          >
            {{ week.monthLabel }}
          </span>
        </div>

        <div class="profile-token-heatmap__weekdays" aria-hidden="true">
          <span
            v-for="(label, index) in WEEKDAY_LABELS"
            :key="`weekday-${index}`"
          >
            {{ label }}
          </span>
        </div>

        <div
          class="profile-token-heatmap__cells"
          :style="{ gridTemplateColumns }"
        >
          <template v-for="week in weeks" :key="week.key">
            <span
              v-for="cell in week.cells"
              :key="cell.key"
              class="profile-token-heatmap__cell"
              :class="{
                'profile-token-heatmap__cell--empty': !cell.day,
                [`profile-token-heatmap__cell--level-${cell.day ? activityLevel(cell.day) : 0}`]:
                  true,
              }"
              :title="cell.day ? cellLabel(cell.day) : undefined"
              :aria-hidden="cell.day ? undefined : true"
              :aria-label="cell.day ? cellLabel(cell.day) : undefined"
              :data-level="cell.day ? activityLevel(cell.day) : undefined"
              :data-test="cell.day ? 'token-heatmap-cell' : undefined"
            />
          </template>
        </div>
      </div>
    </div>

    <div class="profile-token-heatmap__legend" aria-hidden="true">
      <span>Less</span>
      <span class="profile-token-heatmap__cell profile-token-heatmap__cell--level-0" />
      <span class="profile-token-heatmap__cell profile-token-heatmap__cell--level-1" />
      <span class="profile-token-heatmap__cell profile-token-heatmap__cell--level-2" />
      <span class="profile-token-heatmap__cell profile-token-heatmap__cell--level-3" />
      <span class="profile-token-heatmap__cell profile-token-heatmap__cell--level-4" />
      <span>More</span>
    </div>
  </div>
</template>

<style scoped>
.profile-token-heatmap {
  display: grid;
  gap: 14px;
  min-width: 0;
  --profile-token-cell-size: 12px;
  --profile-token-cell-gap: 4px;
}

.profile-token-heatmap__scroll {
  overflow-x: auto;
  padding: 2px 2px 8px;
}

.profile-token-heatmap__grid {
  display: grid;
  grid-template-columns: 28px max-content;
  gap: 6px 10px;
  min-width: max-content;
}

.profile-token-heatmap__month-spacer {
  grid-column: 1;
  grid-row: 1;
}

.profile-token-heatmap__months {
  grid-column: 2;
  grid-row: 1;
  display: grid;
  column-gap: var(--profile-token-cell-gap);
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.72rem;
  font-weight: 650;
  line-height: 1;
}

.profile-token-heatmap__months span {
  min-width: 0;
  overflow: visible;
  white-space: nowrap;
}

.profile-token-heatmap__weekdays {
  grid-column: 1;
  grid-row: 2;
  display: grid;
  grid-template-rows: repeat(7, var(--profile-token-cell-size));
  gap: var(--profile-token-cell-gap);
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.67rem;
  font-weight: 650;
  line-height: var(--profile-token-cell-size);
}

.profile-token-heatmap__cells {
  grid-column: 2;
  grid-row: 2;
  display: grid;
  grid-auto-flow: column;
  grid-template-rows: repeat(7, var(--profile-token-cell-size));
  gap: var(--profile-token-cell-gap);
}

.profile-token-heatmap__cell {
  width: var(--profile-token-cell-size);
  height: var(--profile-token-cell-size);
  border: 1px solid rgba(var(--app-shell-border), 0.34);
  border-radius: 3px;
  background: rgba(var(--app-shell-surface-muted), 0.78);
}

.profile-token-heatmap__cell--empty {
  visibility: hidden;
}

.profile-token-heatmap__cell--level-1 {
  border-color: rgba(var(--v-theme-primary), 0.24);
  background: rgba(var(--v-theme-primary), 0.2);
}

.profile-token-heatmap__cell--level-2 {
  border-color: rgba(var(--v-theme-primary), 0.34);
  background: rgba(var(--v-theme-primary), 0.36);
}

.profile-token-heatmap__cell--level-3 {
  border-color: rgba(var(--v-theme-primary), 0.48);
  background: rgba(var(--v-theme-primary), 0.58);
}

.profile-token-heatmap__cell--level-4 {
  border-color: rgba(var(--v-theme-primary), 0.64);
  background: rgba(var(--v-theme-primary), 0.84);
}

.profile-token-heatmap__legend {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.72rem;
  font-weight: 650;
}

@media (max-width: 720px) {
  .profile-token-heatmap {
    --profile-token-cell-size: 10px;
    --profile-token-cell-gap: 3px;
  }

  .profile-token-heatmap__grid {
    grid-template-columns: 24px max-content;
  }
}
</style>
