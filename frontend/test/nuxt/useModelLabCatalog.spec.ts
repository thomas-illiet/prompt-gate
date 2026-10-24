import { effectScope, nextTick } from 'vue'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useModelLabCatalog } from '../../app/composables/useModelLabCatalog'
import type { ModelLabCatalogResponse } from '../../app/types/model-lab'

const { apiFetch } = vi.hoisted(() => ({ apiFetch: vi.fn() }))

mockNuxtImport('useApiFetch', () => () => apiFetch)

function response(generatedAt: string): ModelLabCatalogResponse {
  return {
    generatedAt,
    limits: {
      maxOutputTokens: 4096,
      maxPromptBytes: 32768,
      maxTargets: 3,
    },
    providers: [],
  }
}

describe('useModelLabCatalog', () => {
  beforeEach(() => apiFetch.mockReset())

  it('keeps the newest response when reloads complete out of order', async () => {
    let resolveFirst!: (value: ModelLabCatalogResponse) => void
    let resolveSecond!: (value: ModelLabCatalogResponse) => void
    apiFetch
      .mockImplementationOnce(
        () =>
          new Promise<ModelLabCatalogResponse>((resolve) => {
            resolveFirst = resolve
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise<ModelLabCatalogResponse>((resolve) => {
            resolveSecond = resolve
          }),
      )

    const scope = effectScope()
    const catalog = scope.run(() => useModelLabCatalog())!
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledOnce())
    const reloadPromise = catalog.reload()
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2))

    resolveSecond(response('2026-10-24T12:00:00Z'))
    await reloadPromise
    resolveFirst(response('2026-10-24T11:00:00Z'))
    await nextTick()

    expect(catalog.catalog.value?.generatedAt).toBe('2026-10-24T12:00:00Z')
    expect(catalog.loading.value).toBe(false)
    expect(
      (apiFetch.mock.calls[0]?.[1] as { signal: AbortSignal }).signal.aborted,
    ).toBe(true)

    scope.stop()
  })
})
