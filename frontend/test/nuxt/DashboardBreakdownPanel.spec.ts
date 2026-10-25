import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardBreakdownPanel from '../../app/components/Dashboard/DashboardBreakdownPanel.vue'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return { apiFetch, useApiFetchMock: vi.fn(() => apiFetch) }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

const VChartStub = defineComponent({
  name: 'VChartStub',
  props: {
    option: {
      required: true,
      type: Object,
    },
  },
  template:
    '<div data-test="chart" :data-axis-overflow="option.yAxis.axisLabel.overflow" :data-axis-width="option.yAxis.axisLabel.width" />',
})

const VBtnToggleStub = defineComponent({
  name: 'VBtnToggleStub',
  props: {
    modelValue: String,
  },
  emits: ['update:modelValue'],
  template: '<div data-test="breakdown-selector"><slot /></div>',
})

const VBtnStub = defineComponent({
  name: 'VBtnStub',
  props: {
    value: String,
  },
  template: '<button :data-value="value"><slot /></button>',
})

function response(name: string) {
  return {
    window: '7d',
    startsAt: '2026-10-15T00:00:00Z',
    endsAt: '2026-10-22T00:00:00Z',
    items: [{ name, requests: 2, totalTokens: 20 }],
  }
}

function mountPanel(
  metric: 'requests' | 'tokens' | 'cost' = 'tokens',
  scope: 'global' | 'self' = 'self',
  allowedBreakdowns?: Array<
    'models' | 'providers' | 'provider-types' | 'identities'
  >,
  customCard?: {
    actions?: string
    icon?: string
    subtitle?: string
    title?: string
  },
) {
  return mount(DashboardBreakdownPanel, {
    props: {
      metric,
      modelValue: 'models',
      refreshVersion: 0,
      scope,
      window: '7d',
      allowedBreakdowns,
      icon: customCard?.icon,
      subtitle: customCard?.subtitle,
      title: customCard?.title,
    },
    slots: customCard?.actions ? { actions: customCard.actions } : {},
    global: {
      stubs: {
        AppSectionCard: {
          props: ['icon', 'subtitle', 'title'],
          template:
            '<section data-test="section-card" :data-icon="icon" :data-subtitle="subtitle" :data-title="title"><header data-test="card-actions"><slot name="actions" /></header><div data-test="card-content"><slot /></div></section>',
        },
        VAlert: { template: '<div><slot /></div>' },
        VBtn: VBtnStub,
        VBtnToggle: VBtnToggleStub,
        Echarts: VChartStub,
        'v-chart': VChartStub,
        VIcon: true,
      },
    },
  })
}

describe('DashboardBreakdownPanel', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
  })

  it('uses the same segmented header selector pattern as the trend card', async () => {
    apiFetch.mockResolvedValueOnce(response('gpt-5'))
    const wrapper = mountPanel()

    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(1))

    const actions = wrapper.get('[data-test="card-actions"]')
    const selector = actions.get('[data-test="breakdown-selector"]')
    expect(selector.attributes('aria-label')).toBe('Usage breakdown')
    expect(
      selector.findAll('button').map((button) => ({
        label: button.attributes('aria-label'),
        value: button.attributes('data-value'),
      })),
    ).toEqual([
      { label: 'Show models breakdown', value: 'models' },
      { label: 'Show providers breakdown', value: 'providers' },
      {
        label: 'Show provider types breakdown',
        value: 'provider-types',
      },
    ])
    expect(
      wrapper.get('[data-test="card-content"] [data-test="chart"]'),
    ).toBeTruthy()
  })

  it('keeps the global identities option in the segmented selector', async () => {
    apiFetch.mockResolvedValueOnce(response('alice@example.com'))
    const wrapper = mountPanel('tokens', 'global')

    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(1))

    const identityButton = wrapper.get(
      '[data-test="breakdown-selector"] [data-value="identities"]',
    )
    expect(identityButton.attributes('aria-label')).toBe(
      'Show identities breakdown',
    )
  })

  it('loads only the active breakdown and switches endpoints on demand', async () => {
    apiFetch
      .mockResolvedValueOnce(response('gpt-5'))
      .mockResolvedValueOnce(response('OpenAI'))
    const wrapper = mountPanel()

    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(1))
    expect(apiFetch.mock.calls[0]?.[0]).toBe(
      '/api/v1/me/dashboard/top-models?window=7d&metric=tokens',
    )
    await vi.waitFor(() => expect(wrapper.text()).toContain('gpt-5 leads'))

    await wrapper.setProps({ modelValue: 'providers' })

    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2))
    expect(apiFetch.mock.calls[1]?.[0]).toBe(
      '/api/v1/me/dashboard/top-provider-names?window=7d&metric=tokens',
    )
    expect(
      apiFetch.mock.calls.some(([url]) =>
        String(url).includes('provider-types'),
      ),
    ).toBe(false)
  })

  it('ranks the loaded entries by the active metric', async () => {
    apiFetch.mockResolvedValueOnce({
      ...response('unused'),
      items: [
        { name: 'Token-heavy', requests: 1, totalTokens: 100 },
        { name: 'Request-heavy', requests: 10, totalTokens: 10 },
      ],
    })
    const wrapper = mountPanel('requests')

    await vi.waitFor(() =>
      expect(wrapper.text()).toContain('Request-heavy leads with 10.'),
    )

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0]?.text()).toContain('Request-heavy')
    expect(rows[1]?.text()).toContain('Token-heavy')
  })

  it('refetches the active breakdown when the metric changes', async () => {
    apiFetch
      .mockResolvedValueOnce(response('Token leader'))
      .mockResolvedValueOnce(response('Request leader'))
    const wrapper = mountPanel('tokens')

    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(1))
    await wrapper.setProps({ metric: 'requests' })

    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2))
    expect(apiFetch.mock.calls[1]?.[0]).toBe(
      '/api/v1/me/dashboard/top-models?window=7d&metric=requests',
    )
  })

  it('bounds long chart axis labels without changing full data labels', async () => {
    const longName = 'automation-service-account-with-a-very-long-name'
    apiFetch.mockResolvedValueOnce(response(longName))
    const wrapper = mountPanel()

    await vi.waitFor(() => expect(wrapper.text()).toContain(longName))

    const chart = wrapper.get('[data-test="chart"]')
    expect(chart.attributes('data-axis-overflow')).toBe('truncate')
    expect(chart.attributes('data-axis-width')).toBe('120')
    expect(wrapper.get('tbody').text()).toContain(longName)
  })

  it('limits tabs to the allowed thematic breakdowns', async () => {
    apiFetch.mockResolvedValueOnce(response('alice@example.com'))
    const wrapper = mountPanel('cost', 'global', ['identities'])

    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledOnce())

    expect(wrapper.find('[data-test="breakdown-selector"]').exists()).toBe(
      false,
    )
    expect(apiFetch.mock.calls[0]?.[0]).toBe(
      '/api/v1/admin/dashboard/top-identities?window=7d&metric=cost',
    )
  })

  it('places custom controls in the card header with thematic copy', async () => {
    apiFetch.mockResolvedValueOnce(response('alice@example.com'))
    const wrapper = mountPanel('requests', 'global', ['identities'], {
      actions:
        '<div data-test="identity-metric-selector">Requests / Tokens / Cost</div>',
      icon: 'mdi-account-multiple-outline',
      subtitle: 'Compare people and services with one metric',
      title: 'Identity ranking',
    })

    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledOnce())

    const card = wrapper.get('[data-test="section-card"]')
    expect(card.attributes('data-icon')).toBe('mdi-account-multiple-outline')
    expect(card.attributes('data-title')).toBe('Identity ranking')
    expect(card.attributes('data-subtitle')).toBe(
      'Compare people and services with one metric',
    )
    expect(
      card.get(
        '[data-test="card-actions"] [data-test="identity-metric-selector"]',
      ),
    ).toBeDefined()
    expect(card.find('[data-test="breakdown-selector"]').exists()).toBe(false)
  })
})
