import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import FAQPage from '../../app/pages/faq.vue'

const { useFAQMock } = vi.hoisted(() => ({
  useFAQMock: vi.fn(),
}))

mockNuxtImport('useFAQ', () => useFAQMock)

function mountPage() {
  return mount(FAQPage, {
    global: {
      stubs: {
        AppPageHeader: {
          props: ['copy', 'statValue', 'visual'],
          template: `
            <div
              data-test="faq-hero"
              :data-copy="copy"
              :data-visual="visual"
            >
              {{ statValue }}
            </div>
          `,
        },
        FAQList: true,
        VAlert: { template: '<div><slot /><slot name="append" /></div>' },
        VBtn: { template: '<button><slot /></button>' },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<div><slot /></div>' },
        VRow: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('FAQPage', () => {
  beforeEach(() => {
    useFAQMock.mockReset()
  })

  it('uses the knowledge visual and an indeterminate count while loading', () => {
    useFAQMock.mockReturnValue({
      entries: shallowRef([]),
      error: shallowRef(null),
      load: vi.fn(),
      loading: shallowRef(true),
    })

    const wrapper = mountPage()
    const hero = wrapper.get('[data-test="faq-hero"]')

    expect(hero.text()).toBe('…')
    expect(hero.attributes('data-visual')).toBe('book')
    expect(hero.attributes('data-copy')).toContain('without the scavenger hunt')
  })

  it('retries a failed FAQ load', async () => {
    const load = vi.fn()
    useFAQMock.mockReturnValue({
      entries: shallowRef([]),
      error: shallowRef('FAQ unavailable.'),
      load,
      loading: shallowRef(false),
    })

    const wrapper = mountPage()

    await wrapper.get('[data-test="faq-retry"]').trigger('click')
    expect(load).toHaveBeenCalledOnce()
  })
})
