import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AccountTableFilters from '../AccountTableFilters.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

const SelectStub = defineComponent({
  name: 'AccountFilterSelectStub',
  props: {
    modelValue: { type: [String, Number, Boolean], default: '' },
    options: { type: Array, default: () => [] }
  },
  emits: ['update:model-value', 'change'],
  template: '<div />'
})

describe('AccountTableFilters Forbidden status', () => {
  it('offers Forbidden and scopes it to the Grok platform', async () => {
    const wrapper = mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: { platform: '', type: '', status: '', privacy_mode: '', group: '' },
        groups: []
      },
      global: {
        stubs: {
          Select: SelectStub,
          SearchInput: true
        }
      }
    })

    const selects = wrapper.findAllComponents(SelectStub)
    const statusSelect = selects[2]
    expect(statusSelect.props('options')).toContainEqual({
      value: 'forbidden',
      label: 'admin.accounts.status.forbidden'
    })

    statusSelect.vm.$emit('update:model-value', 'forbidden')
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('update:filters')?.at(-1)?.[0]).toMatchObject({
      platform: 'grok',
      status: 'forbidden'
    })
  })
})
