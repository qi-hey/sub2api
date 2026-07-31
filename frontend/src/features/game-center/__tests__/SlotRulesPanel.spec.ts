import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import SlotRulesPanel from '../components/SlotRulesPanel.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) => {
      if (key === 'gameCenter.slot.rules.matchSuffix') return '轴'
      if (key === 'gameCenter.slot.rules.freeSpinSuffix') return ' 次免费旋转'
      if (key === 'gameCenter.slot.rules.score') return `得分 ${params?.score}`
      return key
    },
  }),
}))

describe('SlotRulesPanel', () => {
  it('shows all eight symbols and the 243-ways explanation', () => {
    const wrapper = mount(SlotRulesPanel)

    expect(wrapper.findAll('.slot-rules__payrow')).toHaveLength(8)
    expect(wrapper.text()).toContain('gameCenter.slot.symbols.BONUS')
    expect(wrapper.text()).toContain('5轴 得分 400')
    expect(wrapper.text()).toContain('gameCenter.slot.symbols.SCATTER')
    expect(wrapper.text()).toContain('3 × 3 × 3 × 3 × 3 = 243')
    expect(wrapper.text()).toContain('gameCenter.slot.rules.waysDetail')
    expect(wrapper.findAll('.slot-rules__lines figure')).toHaveLength(0)
  })

  it('lists direct scores at bet 10 plus SCATTER scores and free-spin awards', () => {
    const wrapper = mount(SlotRulesPanel)
    const text = wrapper.text()

    expect(text).toContain('3轴 得分 80')
    expect(text).toContain('4轴 得分 200')
    expect(text).toContain('4轴 得分 80')
    expect(text).toContain('4轴 得分 30')
    expect(text).toContain('4轴 得分 20')
    expect(text).toContain('5轴 得分 400')
    expect(text).toContain('5轴 得分 160')
    expect(text).toContain('5轴 得分 54')
    expect(text).toContain('5轴 得分 30')
    expect(text).toContain('5轴 得分 18')
    expect(text).toContain('3轴 得分 5')
    expect(text).toContain('3轴 得分 10 + 5 次免费旋转')
    expect(text).not.toMatch(/\bx\d+/i)
    expect(text).toContain('gameCenter.slot.rules.bonusTrigger')
    expect(text).toContain('gameCenter.slot.rules.specialNote')
  })
})
