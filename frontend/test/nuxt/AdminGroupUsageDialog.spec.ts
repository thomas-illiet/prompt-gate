import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminGroupUsageDialog from '../../app/components/AdminGroups/AdminGroupUsageDialog.vue'
import type { AccessGroup } from '../../app/types/groups'

const group: AccessGroup = {
  id: 'group/id',
  name: 'engineering',
  displayName: 'Engineering',
  description: 'Engineering access',
  providers: [],
  modelPatterns: ['^gpt-5'],
  excludedModelPatterns: [],
  members: [],
  providerCount: 0,
  modelPatternCount: 1,
  memberCount: 0,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function mountDialog(groupOverride: AccessGroup = group) {
  return mount(AdminGroupUsageDialog, {
    props: {
      group: groupOverride,
      modelValue: true,
    },
    global: {
      stubs: {
        AdminAccountUsageDialog: {
          props: [
            'costCaption',
            'emptyUsageText',
            'endpoint',
            'modelValue',
            'subtitle',
            'title',
          ],
          emits: ['update:modelValue'],
          template: `
            <section
              :data-cost-caption="costCaption"
              :data-empty-usage-text="emptyUsageText"
              :data-endpoint="endpoint"
              :data-subtitle="subtitle"
              :data-title="title"
            >
              <button
                data-test="close"
                @click="$emit('update:modelValue', false)"
              >
                Close
              </button>
            </section>
          `,
        },
      },
    },
  })
}

describe('AdminGroupUsageDialog', () => {
  it('adapts a group into the shared usage dialog contract', () => {
    const wrapper = mountDialog()
    const dialog = wrapper.get('section')

    expect(dialog.attributes('data-title')).toBe('Engineering usage statistics')
    expect(dialog.attributes('data-endpoint')).toBe(
      '/api/v1/admin/groups/group%2Fid/statistics',
    )
    expect(dialog.attributes('data-subtitle')).toBe(
      'Usage attributed to the current members of this group across the selected period.',
    )
    expect(dialog.attributes('data-cost-caption')).toBe('Group usage estimate')
    expect(dialog.attributes('data-empty-usage-text')).toBe(
      'This group has no usage in the selected window.',
    )
  })

  it('falls back to the stable group name in the title', () => {
    const wrapper = mountDialog({ ...group, displayName: '' })

    expect(wrapper.get('section').attributes('data-title')).toBe(
      'engineering usage statistics',
    )
  })

  it('forwards close events to its model', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="close"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[false]])
  })
})
