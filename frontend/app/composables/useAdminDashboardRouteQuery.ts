import { shallowRef, watch } from 'vue'

import type {
  DashboardBreakdown,
  DashboardMetric,
  UsageWindow,
} from '~/types/user-service'

export function dashboardQueryValue(value: unknown) {
  return Array.isArray(value) ? String(value[0] ?? '') : String(value ?? '')
}

export function parseDashboardWindow(value: unknown): UsageWindow {
  const normalized = dashboardQueryValue(value)
  return normalized === '30d' || normalized === 'all' ? normalized : '7d'
}

export function parseDashboardMetric(value: unknown): DashboardMetric {
  const normalized = dashboardQueryValue(value)
  return normalized === 'tokens' || normalized === 'cost'
    ? normalized
    : 'requests'
}

export function parseDashboardBreakdown(
  value: unknown,
  allowed: readonly DashboardBreakdown[],
  fallback: DashboardBreakdown,
) {
  const normalized = dashboardQueryValue(value) as DashboardBreakdown
  return allowed.includes(normalized) ? normalized : fallback
}

// useAdminDashboardWindowQuery keeps the shared period in the route query.
export function useAdminDashboardWindowQuery() {
  const route = useRoute()
  const router = useRouter()
  const window = shallowRef<UsageWindow>(
    parseDashboardWindow(route.query.window),
  )
  let syncingFromRoute = false

  function replaceNormalized(value: UsageWindow) {
    if (
      dashboardQueryValue(route.query.window) === value &&
      dashboardQueryValue(route.query.scope) === ''
    ) {
      return
    }
    const query = { ...route.query, window: value }
    Reflect.deleteProperty(query, 'scope')
    void router.replace({ query })
  }

  watch(
    [() => route.query.window, () => route.query.scope],
    ([rawWindow]) => {
      const normalizedWindow = parseDashboardWindow(rawWindow)
      syncingFromRoute = true
      try {
        window.value = normalizedWindow
      } finally {
        syncingFromRoute = false
      }
      replaceNormalized(normalizedWindow)
    },
    { immediate: true },
  )
  watch(
    window,
    (value) => {
      if (!syncingFromRoute) replaceNormalized(value)
    },
    { flush: 'sync' },
  )

  return window
}

// useAdminDashboardAnalysisQuery keeps period, metric, and breakdown normalized.
export function useAdminDashboardAnalysisQuery(
  allowedBreakdowns: readonly DashboardBreakdown[],
  fallbackBreakdown: DashboardBreakdown,
) {
  const route = useRoute()
  const router = useRouter()
  const window = shallowRef<UsageWindow>(
    parseDashboardWindow(route.query.window),
  )
  const metric = shallowRef<DashboardMetric>(
    parseDashboardMetric(route.query.metric),
  )
  const breakdown = shallowRef<DashboardBreakdown>(
    parseDashboardBreakdown(
      route.query.breakdown,
      allowedBreakdowns,
      fallbackBreakdown,
    ),
  )
  let syncingFromRoute = false

  function replaceNormalized(
    nextWindow: UsageWindow,
    nextMetric: DashboardMetric,
    nextBreakdown: DashboardBreakdown,
  ) {
    if (
      dashboardQueryValue(route.query.window) === nextWindow &&
      dashboardQueryValue(route.query.metric) === nextMetric &&
      dashboardQueryValue(route.query.breakdown) === nextBreakdown &&
      dashboardQueryValue(route.query.scope) === ''
    ) {
      return
    }

    const query = {
      ...route.query,
      breakdown: nextBreakdown,
      metric: nextMetric,
      window: nextWindow,
    }
    Reflect.deleteProperty(query, 'scope')
    void router.replace({ query })
  }

  watch(
    [
      () => route.query.window,
      () => route.query.metric,
      () => route.query.breakdown,
      () => route.query.scope,
    ],
    ([rawWindow, rawMetric, rawBreakdown]) => {
      const normalizedWindow = parseDashboardWindow(rawWindow)
      const normalizedMetric = parseDashboardMetric(rawMetric)
      const normalizedBreakdown = parseDashboardBreakdown(
        rawBreakdown,
        allowedBreakdowns,
        fallbackBreakdown,
      )
      syncingFromRoute = true
      try {
        window.value = normalizedWindow
        metric.value = normalizedMetric
        breakdown.value = normalizedBreakdown
      } finally {
        syncingFromRoute = false
      }
      replaceNormalized(normalizedWindow, normalizedMetric, normalizedBreakdown)
    },
    { immediate: true },
  )
  watch(
    [window, metric, breakdown],
    ([nextWindow, nextMetric, nextBreakdown]) => {
      if (!syncingFromRoute) {
        replaceNormalized(nextWindow, nextMetric, nextBreakdown)
      }
    },
    { flush: 'sync' },
  )

  return { breakdown, metric, window }
}
