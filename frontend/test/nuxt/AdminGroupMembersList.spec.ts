import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminGroupMembersList from '../../app/components/AdminGroups/AdminGroupMembersList.vue'
import type { GroupMemberSummary } from '../../app/types/groups'

function member(
  id: string,
  name: string,
  preferredUsername = name.toLowerCase().replaceAll(' ', '.'),
): GroupMemberSummary {
  return {
    id,
    preferredUsername,
    email: `${preferredUsername}@example.com`,
    name,
    type: 'user',
    role: 'user',
    isActive: true,
  }
}

const members: GroupMemberSummary[] = [
  member('member-1', 'Ada Lovelace', 'ada'),
  member('member-2', 'Grace Hopper', 'grace'),
  member('member-3', 'Katherine Johnson', 'katherine'),
  member('member-4', 'Margaret Hamilton', 'margaret'),
  member('member-5', 'Dorothy Vaughan', 'dorothy'),
  member('member-6', 'Mary Jackson', 'mary'),
]

function mountList(items = members) {
  return mount(AdminGroupMembersList, {
    props: {
      active: true,
      loading: false,
      members: items,
    },
    global: {
      stubs: {
        AppEmptyState: {
          props: ['title', 'text'],
          template:
            '<section data-test="empty-state"><span>{{ title }}</span><span>{{ text }}</span></section>',
        },
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: {
          emits: ['click'],
          props: ['ariaLabel', 'loading'],
          template:
            '<button type="button" :aria-label="ariaLabel" :disabled="loading" @click="$emit(\'click\')" />',
        },
        VChip: { template: '<span><slot /></span>' },
        VIcon: { template: '<i />' },
        VList: { template: '<div><slot /></div>' },
        VListItem: {
          props: ['title', 'subtitle'],
          template: `
            <article data-test="member-row">
              <span data-test="member-title">{{ title }}</span>
              <span data-test="member-subtitle">{{ subtitle }}</span>
              <slot name="append" />
            </article>
          `,
        },
        VPagination: {
          emits: ['update:modelValue'],
          props: ['length', 'modelValue'],
          template: `
            <nav data-test="pagination">
              <button
                v-for="page in length"
                :key="page"
                :data-test="'page-' + page"
                type="button"
                @click="$emit('update:modelValue', page)"
              >
                {{ page }}
              </button>
            </nav>
          `,
        },
        VTextField: {
          emits: ['update:modelValue'],
          props: ['modelValue'],
          template:
            '<input data-test="member-search" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
        },
      },
    },
  })
}

describe('AdminGroupMembersList', () => {
  it('paginates members and filters the visible list by search text', async () => {
    const wrapper = mountList()

    expect(wrapper.findAll('[data-test="member-row"]')).toHaveLength(5)
    expect(wrapper.text()).toContain('Ada Lovelace')
    expect(wrapper.text()).not.toContain('Mary Jackson')
    expect(wrapper.find('[data-test="pagination"]').exists()).toBe(true)

    await wrapper.get('[data-test="page-2"]').trigger('click')

    expect(wrapper.findAll('[data-test="member-row"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('Mary Jackson')

    await wrapper.get('[data-test="member-search"]').setValue('grace')

    expect(wrapper.findAll('[data-test="member-row"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('Grace Hopper')
    expect(wrapper.find('[data-test="pagination"]').exists()).toBe(false)
  })

  it('emits remove for the selected member', async () => {
    const wrapper = mountList([members[0] as GroupMemberSummary])

    await wrapper
      .get('button[aria-label="Remove Ada Lovelace"]')
      .trigger('click')

    expect(wrapper.emitted('remove')).toEqual([['member-1']])
  })

  it('hides member search when the group has no members', () => {
    const wrapper = mountList([])

    expect(wrapper.find('[data-test="member-search"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="empty-state"]').text()).toContain(
      'No members',
    )
  })
})
