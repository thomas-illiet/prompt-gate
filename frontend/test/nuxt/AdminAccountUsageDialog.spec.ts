import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, toValue } from 'vue'

import AdminAccountUsageDialog from '../../app/components/AdminAccounts/AdminAccountUsageDialog.vue'

const {
  reload,
  widgetData,
  widgetError,
  widgetLoading,
  useDashboardWidgetMock,
} = vi.hoisted(() => ({
  reload: vi.fn(),
  widgetData: { value: null },
  widgetError: { value: null },
  widgetLoading: { value: false },
  useDashboardWidgetMock: vi.fn(),
}))

mockNuxtImport('useDashboardWidget', () => useDashboardWidgetMock)

const TimeRangeStub = defineComponent({
  props: {
    modelValue: {
      type: String,
      required: true,
    },
  },
  emits: ['update:modelValue'],
  template: `
    <button
      data-test="window-select"
      @click="$emit('update:modelValue', '30d')"
    >
      {{ modelValue }}
    </button>
  `,
})

function mountDialog() {
  return mount(AdminAccountUsageDialog, {
    props: {
      costCaption: 'Account usage estimate',
      emptyUsageText: 'No account usage.',
      endpoint: '/api/v1/admin/accounts/account-id/statistics',
      modelValue: true,
      subtitle: 'Account usage subtitle',
      title: 'Account usage statistics',
    },
    global: {
      stubs: {
        AdminAccountUsageOverview: {
          props: ['costCaption', 'data', 'emptyUsageText', 'error', 'loading'],
          template: '<div data-test="overview" />',
        },
        AppDialogCard: {
          props: ['modelValue', 'subtitle', 'title'],
          emits: ['update:modelValue'],
          template:
            '<section v-if="modelValue"><h2>{{ title }}</h2><p>{{ subtitle }}</p><slot /><footer><slot name="actions" /></footer></section>',
        },
        AppDialogCloseButton: {
          emits: ['click'],
          template:
            '<button data-test="close" @click="$emit(\'click\')">Close</button>',
        },
        DashboardTimeRangeSelect: TimeRangeStub,
        VBtn: {
          emits: ['click'],
          template:
            '<button data-test="refresh" @click="$emit(\'click\')"><slot /></button>',
        },
      },
    },
  })
}

describe('AdminAccountUsageDialog', () => {
  beforeEach(() => {
    reload.mockReset()
    useDashboardWidgetMock.mockReset()
    useDashboardWidgetMock.mockReturnValue({
      data: widgetData,
      error: widgetError,
      loading: widgetLoading,
      reload,
    })
  })

  it('loads the endpoint with a default 7-day window', () => {
    const wrapper = mountDialog()

    expect(wrapper.text()).toContain('Account usage statistics')
    expect(wrapper.text()).toContain('Account usage subtitle')
    expect(useDashboardWidgetMock).toHaveBeenCalledTimes(1)

    const [endpoint, window] = useDashboardWidgetMock.mock.calls[0] ?? []
    expect(toValue(endpoint)).toBe(
      '/api/v1/admin/accounts/account-id/statistics',
    )
    expect(toValue(window)).toBe('7d')
  })

  it('updates the request window and exposes refresh and close actions', async () => {
    const wrapper = mountDialog()
    const [, window] = useDashboardWidgetMock.mock.calls[0] ?? []

    await wrapper.get('[data-test="window-select"]').trigger('click')
    expect(toValue(window)).toBe('30d')

    await wrapper.get('[data-test="refresh"]').trigger('click')
    expect(reload).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-test="close"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[false]])
  })
})
