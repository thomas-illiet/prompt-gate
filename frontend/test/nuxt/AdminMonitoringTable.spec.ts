import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, type PropType } from 'vue'

import AdminMonitoringTable from '../../app/components/AdminMonitoring/AdminMonitoringTable.vue'
import type { MonitoringService } from '../../app/types/monitoring'
import type { AppRowAction } from '../../app/types/row-actions'

const service: MonitoringService = {
  id: 'service-id',
  name: 'api-health',
  displayName: 'API health',
  url: 'https://api.example.com/health',
  expectedStatusCode: 204,
  intervalSeconds: 60,
  enabled: true,
  status: 'degraded',
  lastCheckedAt: '2026-01-01T00:00:00Z',
  lastStatusCode: 500,
  lastError: 'expected HTTP 204, got 500',
  lastLatencyMs: 42,
  consecutiveFailures: 2,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function mountTable() {
  return mount(AdminMonitoringTable, {
    props: {
      items: [service],
      loading: false,
      page: 1,
      pageSize: 10,
      sortBy: 'name',
      sortDir: 'asc',
      total: 1,
    },
    global: {
      stubs: {
        AppRowActionMenu: defineComponent({
          props: {
            actions: {
              type: Array as PropType<AppRowAction<MonitoringService>[]>,
              required: true,
            },
            item: {
              type: Object as PropType<MonitoringService>,
              required: true,
            },
          },
          setup(props) {
            function resolveTitle(action: AppRowAction<MonitoringService>) {
              return typeof action.title === 'function'
                ? action.title(props.item)
                : action.title
            }

            function selectAction(action: AppRowAction<MonitoringService>) {
              action.onSelect?.(props.item)
            }

            return { resolveTitle, selectAction }
          },
          template:
            '<div><button v-for="action in actions" :key="action.key" :data-test="\'row-action-\' + action.key" @click="selectAction(action)">{{ resolveTitle(action) }}</button></div>',
        }),
        AppSectionCard: {
          template: '<section><slot name="actions" /><slot /></section>',
        },
        AppServerDataTable: {
          props: ['items'],
          template:
            '<div><div v-for="item in items" :key="item.id"><slot name="item.name" :item="item" /><slot name="item.actions" :item="item" /></div></div>',
        },
        AppStatusToggleButton: { template: '<button />' },
        VBtn: { template: '<button><slot /></button>' },
        VChip: { template: '<span><slot /></span>' },
      },
    },
  })
}

describe('AdminMonitoringTable', () => {
  it('exposes a details row action', async () => {
    const wrapper = mountTable()

    expect(wrapper.get('[data-test="row-action-details"]').text()).toBe(
      'Details',
    )

    await wrapper.get('[data-test="row-action-details"]').trigger('click')

    expect(wrapper.emitted('details')).toEqual([[service]])
  })
})
