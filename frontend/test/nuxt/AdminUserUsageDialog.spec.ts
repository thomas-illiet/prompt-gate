import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, toValue } from 'vue'

import AdminUserUsageDialog from '../../app/components/AdminUsers/AdminUserUsageDialog.vue'
import type { AdminUser } from '../../app/types/users'

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

const user: AdminUser = {
  id: 'user/id',
  sub: 'oidc-sub',
  preferredUsername: 'ada',
  email: 'ada@example.com',
  name: 'Ada Lovelace',
  role: 'user',
  note: '',
  isActive: true,
  firewallOverrideEnabled: false,
  lastLoginAt: '2026-01-02T00:00:00Z',
  inputTokens: 123,
  outputTokens: 456,
  expiresAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

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
  return mount(AdminUserUsageDialog, {
    props: {
      modelValue: true,
      user,
    },
    global: {
      stubs: {
        AdminUserUsageOverview: {
          props: ['data', 'error', 'loading'],
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

describe('AdminUserUsageDialog', () => {
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

  it('targets the selected user and defaults to the 7-day statistics endpoint', () => {
    const wrapper = mountDialog()

    expect(wrapper.text()).toContain('Ada Lovelace usage statistics')
    expect(useDashboardWidgetMock).toHaveBeenCalledTimes(1)

    const [endpoint, window] = useDashboardWidgetMock.mock.calls[0] ?? []
    expect(toValue(endpoint)).toBe('/api/v1/admin/users/user%2Fid/statistics')
    expect(toValue(window)).toBe('7d')
  })

  it('updates the request window and exposes a manual refresh', async () => {
    const wrapper = mountDialog()
    const [, window] = useDashboardWidgetMock.mock.calls[0] ?? []

    await wrapper.get('[data-test="window-select"]').trigger('click')

    expect(toValue(window)).toBe('30d')

    await wrapper.get('[data-test="refresh"]').trigger('click')

    expect(reload).toHaveBeenCalledTimes(1)
  })
})
