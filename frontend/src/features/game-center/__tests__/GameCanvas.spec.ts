import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import GameCanvas from '../components/GameCanvas.vue'

const { createArcadeGame } = vi.hoisted(() => ({
  createArcadeGame: vi.fn(),
}))

vi.mock('@/features/game-center/audio', () => ({
  SOUND_EVENT: 'sub2api-game-sound',
  ArcadeAudio: class {
    readonly isMuted = false

    attachUnlockListeners(): void {}
    destroy(): void {}
    play(): void {}
    preload(): void {}
    async playSynchronized(): Promise<{ startedAtMs: number; durationMs: number | null }> {
      return { startedAtMs: 0, durationMs: null }
    }
    setMuted(): void {}
    async unlock(): Promise<void> {}
  },
}))

vi.mock('@/features/game-center/phaser/createArcadeGame', () => ({
  createArcadeGame,
}))

vi.mock('@/features/game-center/phaser/ArcadeScene', () => ({
  CONTROL_EVENT: 'sub2api-game-control',
}))

function createGameInstance() {
  return {
    events: {
      emit: vi.fn(() => true),
      on: vi.fn(),
      off: vi.fn(),
    },
    destroy: vi.fn(),
  }
}

function mountCanvas(attachTo?: HTMLElement) {
  return mount(GameCanvas, {
    props: {
      gameId: 'tetris',
      label: 'Tetris',
      controlsDescription: 'Use the arrow keys to move and P to pause.',
    },
    global: {
      stubs: {
        Icon: true,
      },
    },
    attachTo,
  })
}

describe('GameCanvas keyboard and accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    createArcadeGame.mockReturnValue(createGameInstance())
  })

  it('describes the application with the supplied controls text', async () => {
    const wrapper = mountCanvas()
    await flushPromises()

    const application = wrapper.get('[role="application"]')
    const descriptionId = application.attributes('aria-describedby')

    expect(descriptionId).toBe('game-canvas-tetris-controls')
    expect(wrapper.get(`#${descriptionId}`).text()).toBe(
      'Use the arrow keys to move and P to pause.',
    )
  })

  it('prevents focused arrow keys from scrolling while leaving them available to Phaser', async () => {
    const wrapper = mountCanvas(document.body)
    await flushPromises()
    const application = wrapper.get('[role="application"]')
    const phaserKeydown = vi.fn()
    window.addEventListener('keydown', phaserKeydown)

    try {
      const applicationElement = application.element as HTMLElement
      applicationElement.focus()
      const event = new KeyboardEvent('keydown', {
        key: 'ArrowDown',
        bubbles: true,
        cancelable: true,
      })
      application.element.dispatchEvent(event)

      expect(event.defaultPrevented).toBe(true)
      expect(phaserKeydown).toHaveBeenCalledTimes(1)
    } finally {
      window.removeEventListener('keydown', phaserKeydown)
      wrapper.unmount()
    }
  })

  it('sends one pause control for each P press without reaching Phaser key listeners', async () => {
    const game = createGameInstance()
    createArcadeGame.mockReturnValue(game)
    const wrapper = mountCanvas(document.body)
    await flushPromises()
    const application = wrapper.get('[role="application"]')
    const phaserKeydown = vi.fn()
    window.addEventListener('keydown', phaserKeydown)

    try {
      application.element.dispatchEvent(new KeyboardEvent('keydown', {
        key: 'p',
        code: 'KeyP',
        bubbles: true,
        cancelable: true,
      }))
      application.element.dispatchEvent(new KeyboardEvent('keydown', {
        key: 'p',
        code: 'KeyP',
        repeat: true,
        bubbles: true,
        cancelable: true,
      }))
      application.element.dispatchEvent(new KeyboardEvent('keydown', {
        key: 'P',
        code: 'KeyP',
        bubbles: true,
        cancelable: true,
      }))

      expect(game.events.emit).toHaveBeenCalledTimes(2)
      expect(game.events.emit).toHaveBeenNthCalledWith(1, 'sub2api-game-control', 'pause')
      expect(game.events.emit).toHaveBeenNthCalledWith(2, 'sub2api-game-control', 'pause')
      expect(phaserKeydown).not.toHaveBeenCalled()
    } finally {
      window.removeEventListener('keydown', phaserKeydown)
      wrapper.unmount()
    }
  })

  it('passes the one-shot slot bonus hooks through to Phaser', async () => {
    const bonusResult = {
      round_id: 'bonus-1',
      kind: 'free_spins' as const,
      choice: 1,
      reward_credits: '66',
      credits_after: '966',
      claimed_at: '2026-07-28T04:00:00Z',
      spins: 12,
      multiplier: 3,
      bonus_multiplier: 2,
      total_base_payout: '11',
      spin_results: [],
    }
    const requestSlotBonus = vi.fn(async () => bonusResult)
    const formatSlotError = vi.fn(() => '领取失败')
    const wrapper = mount(GameCanvas, {
      props: {
        gameId: 'lucky',
        label: 'Lucky',
        controlsDescription: 'Tap spin.',
        requestSlotBonus,
        formatSlotError,
      },
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    const hooks = createArcadeGame.mock.calls.at(-1)?.[2]
    await expect(hooks.requestSlotBonus('token', 1)).resolves.toEqual(bonusResult)
    expect(requestSlotBonus).toHaveBeenCalledOnce()
    expect(hooks.formatSlotError(new Error('failed'))).toBe('领取失败')

    hooks.onSlotBonusResult(bonusResult)
    expect(wrapper.emitted('slot-bonus-result')).toEqual([[bonusResult]])
  })

  it('passes authoritative fruit hooks through to Phaser', async () => {
    const result = {
      id: 'fruit-1',
      bets: { bar: '1', '77': '0', star: '0', watermelon: '0', bell: '0', mango: '0', orange: '0', apple: '0' },
      total_bet: '1',
      stop_index: 0,
      outcome: { kind: 'fruit' as const, symbol: 'bar' as const, size: 'big' as const, multiplier: '100' },
      payout: '100',
      credits_before: '500',
      credits_after: '599',
      paytable_version: 'fruit-v1',
      idempotency_key: 'fruit-key',
      created_at: '2026-07-29T00:00:00Z',
    }
    const requestFruitSpin = vi.fn(async () => result)
    const getFruitState = vi.fn(() => ({ credits: '500', loyaltyEnabled: true, loyaltyAvailable: true }))
    const wrapper = mount(GameCanvas, {
      props: {
        gameId: 'merge2048', label: '欢乐水果机', controlsDescription: '点击下注并启动。',
        requestFruitSpin, getFruitState,
      },
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    const hooks = createArcadeGame.mock.calls.at(-1)?.[2]
    await expect(hooks.requestFruitSpin(result.bets)).resolves.toEqual(result)
    expect(hooks.getFruitState()).toEqual({ credits: '500', loyaltyEnabled: true, loyaltyAvailable: true })
    hooks.onFruitResult(result)
    expect(wrapper.emitted('fruit-result')).toEqual([[result]])
  })
})
