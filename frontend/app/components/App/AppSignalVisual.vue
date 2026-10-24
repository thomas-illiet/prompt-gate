<script setup lang="ts">
import type { AppSignalVisualName } from '~/types/theme'

const props = withDefaults(
  defineProps<{
    label?: string
    name: AppSignalVisualName
  }>(),
  {
    label: undefined,
  },
)
</script>

<template>
  <svg
    class="app-signal-visual"
    viewBox="0 0 160 160"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    :data-signal="props.name"
    :aria-hidden="props.label ? undefined : 'true'"
    :aria-label="props.label"
    :role="props.label ? 'img' : undefined"
  >
    <circle class="app-signal-visual__halo" cx="80" cy="80" r="62" />
    <circle class="app-signal-visual__core" cx="80" cy="80" r="43" />

    <g v-if="props.name === 'gate'" class="app-signal-visual__glyph">
      <path d="M54 113V65c0-19 11-32 26-32s26 13 26 32v48" />
      <path d="M67 113V68c0-10 5-17 13-17s13 7 13 17v45" />
      <path d="M39 113h82" />
      <path d="m80 76 10 9-10 9-10-9 10-9Z" />
    </g>

    <g v-else-if="props.name === 'radar'" class="app-signal-visual__glyph">
      <circle cx="80" cy="80" r="34" />
      <circle cx="80" cy="80" r="20" />
      <path d="M80 80 105 55" />
      <path d="M80 36v88M36 80h88" />
      <circle class="app-signal-visual__accent" cx="105" cy="55" r="6" />
    </g>

    <g v-else-if="props.name === 'key'" class="app-signal-visual__glyph">
      <circle cx="63" cy="72" r="21" />
      <circle cx="63" cy="72" r="7" />
      <path d="m78 87 32 32" />
      <path d="m94 103 10-10M104 113l10-10" />
      <path d="M45 113h35" />
    </g>

    <g v-else-if="props.name === 'cables'" class="app-signal-visual__glyph">
      <path d="M49 46v20c0 8 6 14 14 14h34c8 0 14 6 14 14v20" />
      <path d="M111 46v14c0 8-6 14-14 14H69c-8 0-14 6-14 14v26" />
      <rect x="40" y="34" width="18" height="14" rx="4" />
      <rect x="102" y="34" width="18" height="14" rx="4" />
      <path d="M45 34V25M53 34V25M107 34V25M115 34V25" />
      <circle class="app-signal-visual__accent" cx="55" cy="118" r="7" />
      <circle class="app-signal-visual__accent" cx="111" cy="118" r="7" />
    </g>

    <g v-else-if="props.name === 'heartbeat'" class="app-signal-visual__glyph">
      <path d="M34 82h25l10-24 17 49 12-31 8 6h20" />
      <path d="M51 42a48 48 0 1 1-7 65" />
      <circle class="app-signal-visual__accent" cx="69" cy="58" r="5" />
    </g>

    <g v-else-if="props.name === 'network'" class="app-signal-visual__glyph">
      <path d="M61 62h38M60 66l16 30m24-30L84 96" />
      <circle cx="54" cy="58" r="10" />
      <circle cx="106" cy="58" r="10" />
      <circle cx="80" cy="104" r="11" />
      <circle class="app-signal-visual__accent" cx="80" cy="80" r="5" />
    </g>

    <g v-else-if="props.name === 'people'" class="app-signal-visual__glyph">
      <circle cx="68" cy="65" r="11" />
      <circle cx="97" cy="70" r="8" />
      <path d="M48 108c2-19 12-29 26-29s24 10 26 29" />
      <path d="M94 83c12 1 19 9 20 24" />
      <circle class="app-signal-visual__accent" cx="48" cy="53" r="5" />
    </g>

    <g v-else-if="props.name === 'ledger'" class="app-signal-visual__glyph">
      <rect x="48" y="45" width="64" height="70" rx="10" />
      <path d="M61 63h37M61 78h25M61 94h17" />
      <circle class="app-signal-visual__accent" cx="99" cy="97" r="11" />
      <path d="M99 91v12m-4-9h7m-7 6h7" />
    </g>

    <g v-else-if="props.name === 'book'" class="app-signal-visual__glyph">
      <path d="M42 50h25c8 0 13 5 13 13v50c0-8-5-12-13-12H42V50Z" />
      <path d="M118 50H93c-8 0-13 5-13 13v50c0-8 5-12 13-12h25V50Z" />
      <path d="M53 66h16M91 66h16M53 80h16M91 80h16" />
      <circle class="app-signal-visual__accent" cx="80" cy="113" r="5" />
    </g>

    <g v-else-if="props.name === 'identity'" class="app-signal-visual__glyph">
      <rect x="43" y="48" width="74" height="64" rx="11" />
      <circle cx="68" cy="72" r="10" />
      <path d="M52 99c2-11 8-17 16-17s14 6 16 17" />
      <path d="M91 68h14M91 82h14M91 96h10" />
      <circle class="app-signal-visual__accent" cx="111" cy="48" r="6" />
    </g>

    <g v-else-if="props.name === 'robot'" class="app-signal-visual__glyph">
      <path d="M80 47V37" />
      <rect x="48" y="50" width="64" height="58" rx="12" />
      <path d="M48 70h-8v20h8m64-20h8v20h-8" />
      <circle cx="68" cy="75" r="5" />
      <circle cx="92" cy="75" r="5" />
      <path d="M66 94h28" />
      <circle class="app-signal-visual__accent" cx="80" cy="34" r="5" />
    </g>

    <g v-else class="app-signal-visual__glyph">
      <circle cx="80" cy="80" r="25" />
      <path d="M68 80h24M80 68v24" />
    </g>

    <circle class="app-signal-visual__dot" cx="31" cy="64" r="3" />
    <circle class="app-signal-visual__dot" cx="126" cy="111" r="3" />
  </svg>
</template>

<style scoped>
.app-signal-visual {
  display: block;
  width: 100%;
  height: 100%;
  color: rgb(var(--v-theme-primary));
  overflow: visible;
}

.app-signal-visual__halo {
  fill: rgba(var(--v-theme-primary), 0.04);
  stroke: rgba(var(--v-theme-primary), 0.18);
  stroke-dasharray: 4 8;
  transform-origin: 80px 80px;
  animation: app-signal-orbit 18s linear infinite;
}

.app-signal-visual__core {
  fill: rgba(var(--app-shell-surface-strong), 0.76);
  stroke: rgba(var(--v-theme-primary), 0.2);
}

.app-signal-visual__glyph {
  stroke: currentColor;
  stroke-width: 5;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.app-signal-visual__accent {
  fill: rgb(var(--app-accent-coral));
  stroke: rgb(var(--app-shell-surface-strong));
  stroke-width: 3;
}

.app-signal-visual__dot {
  fill: rgba(var(--v-theme-primary), 0.62);
}

@keyframes app-signal-orbit {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-signal-visual__halo {
    animation: none;
  }
}
</style>
