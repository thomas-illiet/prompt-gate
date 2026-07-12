import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAdminSetupGuides } from '../../app/composables/useAdminSetupGuides'
import type {
  SetupGuide,
  SetupGuidePayload,
} from '../../app/types/setup-guides'

const guide: SetupGuide = {
  id: 'guide-id',
  identifier: 'python',
  title: 'Python',
  subtitle: 'Python SDK',
  icon: 'mdi-language-python',
  compatibility: 'openai',
  modelMode: 'single',
  filePaths: ['main.py'],
  template: 'original {{model}}',
  enabled: true,
  position: 0,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const { apiFetch, useApiFetchMock } = vi.hoisted(() => ({
  apiFetch: vi.fn(),
  useApiFetchMock: vi.fn(),
}))

mockNuxtImport('useApiFetch', () => useApiFetchMock)

describe('useAdminSetupGuides', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockReset()
    useApiFetchMock.mockReturnValue(apiFetch)
  })

  it('updates the selected guide template', async () => {
    apiFetch
      .mockResolvedValueOnce({ ...guide, template: 'updated {{model}}' })
      .mockResolvedValueOnce({
        items: [{ ...guide, template: 'updated {{model}}' }],
      })
    const admin = useAdminSetupGuides()
    const payload: SetupGuidePayload = {
      ...guide,
      template: 'updated {{model}}',
    }

    admin.selectedGuide.value = guide
    await admin.save(payload)

    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/admin/setup-guides/guide-id',
      { method: 'PATCH', body: payload },
    )
    expect(admin.guides.value[0]?.template).toBe('updated {{model}}')
  })
})
