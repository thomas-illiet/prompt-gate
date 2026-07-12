import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'

import { useFAQ } from '../../app/composables/useFAQ'

const { apiFetch } = vi.hoisted(() => ({ apiFetch: vi.fn() }))
mockNuxtImport('useApiFetch', () => () => apiFetch)

describe('useFAQ', () => {
  it('loads published FAQ entries on mount', async () => {
    apiFetch.mockResolvedValueOnce([{ id: '1', question: 'Question?', renderedHtml: '<p>Answer</p>', position: 0 }])
    const holder = {} as { state: ReturnType<typeof useFAQ> }
    const wrapper = mount(defineComponent({ setup() { holder.state = useFAQ(); return () => null } }))
    await flushPromises()
    expect(apiFetch).toHaveBeenCalledWith('/api/v1/faq')
    expect(holder.state.entries.value).toHaveLength(1)
    expect(holder.state.loading.value).toBe(false)
    wrapper.unmount()
  })
})
