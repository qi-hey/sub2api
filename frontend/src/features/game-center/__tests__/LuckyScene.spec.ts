import { beforeEach, describe, expect, it, vi } from 'vitest'

const { restart } = vi.hoisted(() => ({ restart: vi.fn() }))

vi.mock('phaser', () => {
  class Scene {
    scene = {
      restart,
      pause: vi.fn(),
      resume: vi.fn(),
    }
  }

  return {
    default: {
      Scene,
      Scenes: { Events: { SHUTDOWN: 'shutdown' } },
      Math: {
        Between: (min: number) => min,
        FloatBetween: (min: number) => min,
      },
    },
  }
})

import { LuckyScene } from '../phaser/scenes/LuckyScene'
import { normalizeServerGrid, SLOT_SYMBOL_CODES } from '../slotSymbols'
import type { SceneHooks } from '../types'

const hooks: SceneHooks = {
  onScore: vi.fn(),
  onStatus: vi.fn(),
}

describe('LuckyScene', () => {
  beforeEach(() => {
    restart.mockClear()
  })

  it('blocks cabinet restart while a spin or bonus claim is settling', () => {
    const scene = new LuckyScene(hooks)
    const state = scene as unknown as {
      spinning: boolean
      bonusClaiming: boolean
      handleControl: (control: 'restart') => void
    }

    state.handleControl('restart')
    expect(restart).toHaveBeenCalledTimes(1)

    state.spinning = true
    state.handleControl('restart')
    state.spinning = false
    state.bonusClaiming = true
    state.handleControl('restart')

    expect(restart).toHaveBeenCalledTimes(1)
  })

  it('preserves the R23 BONUS symbol in a server grid', () => {
    expect(SLOT_SYMBOL_CODES).toContain('BONUS')
    expect(normalizeServerGrid([
      ['BONUS', 'JP', 'DR', 'IG', 'JA'],
      ['BE', 'WILD', 'SCATTER', 'BONUS', 'JP'],
      ['DR', 'IG', 'JA', 'BE', 'BONUS'],
    ])).toEqual([
      ['BONUS', 'JP', 'DR', 'IG', 'JA'],
      ['BE', 'WILD', 'SCATTER', 'BONUS', 'JP'],
      ['DR', 'IG', 'JA', 'BE', 'BONUS'],
    ])
  })

  it('classifies regular, big, and jackpot presentation tiers without changing payouts', () => {
    const scene = new LuckyScene(hooks)
    const state = scene as unknown as {
      resolveWinCelebrationTier: (payout: string, bet: string) => 'regular' | 'big' | 'jackpot'
    }

    expect(state.resolveWinCelebrationTier('249', '10')).toBe('regular')
    expect(state.resolveWinCelebrationTier('250', '10')).toBe('big')
    expect(state.resolveWinCelebrationTier('599', '10')).toBe('big')
    expect(state.resolveWinCelebrationTier('600', '10')).toBe('jackpot')
  })

  it.each([
    ['regular', 14],
    ['big', 46],
    ['jackpot', 78],
  ] as const)('uses a bounded %s coin rain with %i reusable sprites', (tier, expectedCount) => {
    const scene = new LuckyScene(hooks)
    const image = vi.fn(() => {
      const coin = {
        active: true,
        angle: 0,
        setDepth: vi.fn(),
        setScale: vi.fn(),
        setAngle: vi.fn(),
        destroy: vi.fn(),
      }
      coin.setDepth.mockReturnValue(coin)
      coin.setScale.mockReturnValue(coin)
      coin.setAngle.mockReturnValue(coin)
      return coin
    })
    const addTween = vi.fn()
    const state = scene as unknown as {
      reducedMotion: boolean
      add: { image: typeof image }
      tweens: { add: typeof addTween }
      ensureCoinTexture: ReturnType<typeof vi.fn>
      spawnCoinRain: (value: typeof tier) => void
    }
    state.reducedMotion = false
    state.add = { image }
    state.tweens = { add: addTween }
    state.ensureCoinTexture = vi.fn()

    state.spawnCoinRain(tier)

    expect(state.ensureCoinTexture).toHaveBeenCalledOnce()
    expect(image).toHaveBeenCalledTimes(expectedCount)
    expect(addTween).toHaveBeenCalledTimes(expectedCount)
    for (const [config] of addTween.mock.calls) expect(config.repeat).toBeUndefined()
  })

  it('suppresses coin rain when reduced motion is requested', () => {
    const scene = new LuckyScene(hooks)
    const state = scene as unknown as {
      reducedMotion: boolean
      ensureCoinTexture: ReturnType<typeof vi.fn>
      spawnCoinRain: (tier: 'jackpot') => void
    }
    state.reducedMotion = true
    state.ensureCoinTexture = vi.fn()

    state.spawnCoinRain('jackpot')

    expect(state.ensureCoinTexture).not.toHaveBeenCalled()
  })

  it('claims a free-spin plan and starts server-result playback', async () => {
    const result = {
      round_id: 'round-1',
      kind: 'free_spins' as const,
      choice: 0,
      reward_credits: '88',
      credits_after: '988',
      claimed_at: '2026-07-29T01:00:00Z',
      spins: 8,
      multiplier: 5,
      bonus_multiplier: 2,
      total_base_payout: '9',
      spin_results: [],
    }
    const requestSlotBonus = vi.fn(async () => result)
    const scene = new LuckyScene({ ...hooks, requestSlotBonus })
    const choiceContainer = {
      disableInteractive: vi.fn(),
      setScale: vi.fn(),
    }
    choiceContainer.disableInteractive.mockReturnValue(choiceContainer)
    choiceContainer.setScale.mockReturnValue(choiceContainer)
    const playBonusSpins = vi.fn(async () => undefined)
    const finishBonusResult = vi.fn()
    const state = scene as unknown as {
      bonusRound: unknown
      claimBonus: (choice: number) => Promise<void>
      playBonusSpins: typeof playBonusSpins
      finishBonusResult: typeof finishBonusResult
      emitSound: ReturnType<typeof vi.fn>
    }
    state.bonusRound = {
      token: 'server-token',
      offer: {
        token: 'server-token', kind: 'free_spins', expiresAt: '', betCredits: '10',
        bonusCount: 4, bonusMultiplier: 2,
        options: [{ choice: 0, spins: 8, multiplier: 5 }],
      },
      generation: 1,
      message: { setText: vi.fn() },
      returnButton: { setVisible: vi.fn() },
      choices: [{
        container: choiceContainer,
        background: { setFillStyle: vi.fn() },
        label: { setText: vi.fn() },
        option: { choice: 0, spins: 8, multiplier: 5 },
      }],
    }
    state.playBonusSpins = playBonusSpins
    state.finishBonusResult = finishBonusResult
    state.emitSound = vi.fn()

    await state.claimBonus(0)

    expect(requestSlotBonus).toHaveBeenCalledWith('server-token', 0)
    expect(playBonusSpins).toHaveBeenCalledWith(result, state.bonusRound)
    expect(finishBonusResult).toHaveBeenCalledWith(result)
  })

  it('plays the dedicated celebration cue when the server awards free spins', () => {
    const scene = new LuckyScene(hooks)
    const emitSound = vi.fn()
    const messageText = { text: '', setText: vi.fn((text: string) => { messageText.text = text }) }
    const state = scene as unknown as {
      spinGeneration: number
      creditText: { setText: ReturnType<typeof vi.fn> }
      freeSpinText: { setText: ReturnType<typeof vi.fn> }
      betText: { setText: ReturnType<typeof vi.fn> }
      messageText: typeof messageText
      applyGrid: ReturnType<typeof vi.fn>
      highlightWinningLines: ReturnType<typeof vi.fn>
      setSpinButtonEnabled: ReturnType<typeof vi.fn>
      hooksOnResult: ReturnType<typeof vi.fn>
      copy: (key: string) => string
      emitSound: typeof emitSound
      finishSpin: (result: import('../types').SlotSpinServerResult, generation: number) => void
    }
    state.spinGeneration = 7
    state.creditText = { setText: vi.fn() }
    state.freeSpinText = { setText: vi.fn() }
    state.betText = { setText: vi.fn() }
    state.messageText = messageText
    state.applyGrid = vi.fn()
    state.highlightWinningLines = vi.fn()
    state.setSpinButtonEnabled = vi.fn()
    state.hooksOnResult = vi.fn()
    state.copy = (key) => key
    state.emitSound = emitSound

    state.finishSpin({
      id: 'spin-free',
      bet_credits: '10',
      payout_credits: '0',
      used_free_spin: false,
      grid: [
        ['SCATTER', 'JP', 'DR', 'IG', 'JA'],
        ['SCATTER', 'BE', 'WILD', 'JP', 'DR'],
        ['SCATTER', 'IG', 'JA', 'BE', 'JP'],
      ],
      stops: [0, 1, 2, 3, 4],
      winning_lines: [],
      scatter_count: 3,
      free_spins_awarded: 8,
      free_spins_remaining: 8,
      credits_before: '100',
      credits_after: '100',
      paytable_version: 'r23',
      reel_strip_version: 'r23',
      idempotency_key: 'spin-free-key',
      created_at: '2026-07-29T02:00:00Z',
    }, 7)

    expect(emitSound).toHaveBeenNthCalledWith(1, 'slots-no-win')
    expect(emitSound).toHaveBeenNthCalledWith(2, 'slots-free-spins')
  })

  it('restores a server-owned pending bonus when the wallet is synchronized', () => {
    const pendingSlotBonus = {
      token: 'restored-server-token',
      kind: 'free_spins' as const,
      expiresAt: '2026-07-29T03:10:00Z',
      betCredits: '10',
      bonusCount: 5,
      bonusMultiplier: 4,
      options: [
        { choice: 0, spins: 8, multiplier: 5 },
        { choice: 1, spins: 12, multiplier: 3 },
        { choice: 2, spins: 20, multiplier: 2 },
      ],
    }
    const scene = new LuckyScene({
      ...hooks,
      getSlotState: () => ({
        credits: '900',
        freeSpinsRemaining: 0,
        betCredits: '10',
        loyaltyEnabled: true,
        loyaltyAvailable: true,
        pendingSlotBonus,
      }),
      requestSlotBonus: vi.fn(),
    })
    const enterBonusRound = vi.fn()
    const state = scene as unknown as {
      refreshWalletHud: ReturnType<typeof vi.fn>
      enterBonusRound: typeof enterBonusRound
    }
    state.refreshWalletHud = vi.fn()
    state.enterBonusRound = enterBonusRound

    scene.syncWalletState()

    expect(enterBonusRound).toHaveBeenCalledWith(pendingSlotBonus)
  })

  it('keeps the pending token and retry controls after a claim failure', async () => {
    const requestSlotBonus = vi.fn().mockRejectedValue({ status: 503 })
    const scene = new LuckyScene({ ...hooks, requestSlotBonus })
    const returnButton = { setVisible: vi.fn() }
    const choiceContainer = {
      disableInteractive: vi.fn(),
      setScale: vi.fn(),
      setInteractive: vi.fn(),
    }
    choiceContainer.disableInteractive.mockReturnValue(choiceContainer)
    choiceContainer.setScale.mockReturnValue(choiceContainer)
    const background = { setFillStyle: vi.fn() }
    const round = {
      token: 'retry-server-token',
      offer: {
        token: 'retry-server-token', kind: 'free_spins' as const, expiresAt: '', betCredits: '10',
        bonusCount: 3, bonusMultiplier: 1,
        options: [{ choice: 1, spins: 12, multiplier: 3 }],
      },
      generation: 1,
      message: { setText: vi.fn() },
      returnButton,
      choices: [{
        container: choiceContainer,
        background,
        label: { setText: vi.fn() },
        option: { choice: 1, spins: 12, multiplier: 3 },
      }],
    }
    const state = scene as unknown as {
      bonusRound: typeof round | null
      claimBonus: (choice: number) => Promise<void>
      emitSound: ReturnType<typeof vi.fn>
    }
    state.bonusRound = round
    state.emitSound = vi.fn()

    await state.claimBonus(1)

    expect(state.bonusRound?.token).toBe('retry-server-token')
    expect(choiceContainer.setInteractive).toHaveBeenCalled()
    expect(returnButton.setVisible).toHaveBeenLastCalledWith(false)
  })

  it('summarizes each symbol once and shows the final BONUS-adjusted score', () => {
    const scene = new LuckyScene(hooks)
    const state = scene as unknown as {
      formatWinningSummary: (
        lines: import('../types').SlotWinningLine[],
        payoutCredits?: string,
        bonusCount?: number,
        fifthReelHasBonus?: boolean,
      ) => string | null
    }

    expect(state.formatWinningSummary([
      { line_index: 1, symbol: 'IG', count: 5, multiplier: '50', payout: '5', positions: [] },
      { line_index: 2, symbol: 'JA', count: 4, multiplier: '20', payout: '4', positions: [] },
      { line_index: 3, symbol: 'JA', count: 4, multiplier: '20', payout: '6', positions: [] },
    ])).toBe('元宝5轴 + 玉牌4轴 = 15分')

    expect(state.formatWinningSummary([
      { line_index: 1, symbol: 'DR', count: 4, multiplier: '160', payout: '160', positions: [] },
    ], '240', 1, true)).toBe('龙4轴；1 个彩金 ×1.5 = 240分')

    expect(state.formatWinningSummary([
      { line_index: 1, symbol: 'DR', count: 4, multiplier: '160', payout: '160', positions: [] },
    ], '160', 1, false)).toBe('龙4轴 = 160分')
  })

  it('shows a creation failure instead of no-win when BONUS has no round offer', () => {
    const scene = new LuckyScene(hooks)
    const messageText = { text: '', setText: vi.fn((text: string) => { messageText.text = text }) }
    const state = scene as unknown as {
      spinGeneration: number
      creditText: { setText: ReturnType<typeof vi.fn> }
      freeSpinText: { setText: ReturnType<typeof vi.fn> }
      betText: { setText: ReturnType<typeof vi.fn> }
      messageText: typeof messageText
      applyGrid: ReturnType<typeof vi.fn>
      highlightWinningLines: ReturnType<typeof vi.fn>
      setSpinButtonEnabled: ReturnType<typeof vi.fn>
      hooksOnResult: ReturnType<typeof vi.fn>
      emitSound: ReturnType<typeof vi.fn>
      finishSpin: (result: import('../types').SlotSpinServerResult, generation: number) => void
    }
    state.spinGeneration = 3
    state.creditText = { setText: vi.fn() }
    state.freeSpinText = { setText: vi.fn() }
    state.betText = { setText: vi.fn() }
    state.messageText = messageText
    state.applyGrid = vi.fn()
    state.highlightWinningLines = vi.fn()
    state.setSpinButtonEnabled = vi.fn()
    state.hooksOnResult = vi.fn()
    state.emitSound = vi.fn()

    state.finishSpin({
      id: 'spin-missing-bonus', bet_credits: '10', payout_credits: '0', used_free_spin: false,
      grid: normalizeServerGrid([]), stops: [], winning_lines: [], scatter_count: 0, bonus_count: 3,
      free_spins_awarded: 0, free_spins_remaining: 0, credits_before: '100', credits_after: '90',
      paytable_version: 'slot-paytable-v5', reel_strip_version: 'slot-reels-v4',
      idempotency_key: 'missing-bonus-key', created_at: '2026-07-29T04:00:00Z',
    }, 3)

    expect(messageText.setText).toHaveBeenLastCalledWith('奖励局创建失败，请重试')
  })

  it('locks ordinary free spins to the wallet default bet', () => {
    const scene = new LuckyScene({
      ...hooks,
      getSlotState: () => ({
        credits: '100', freeSpinsRemaining: 2, betCredits: '5',
        loyaltyEnabled: true, loyaltyAvailable: true,
      }),
    })
    const state = scene as unknown as {
      selectedBetCredits: string
      walletDefaultBetCredits: string
      betInitialized: boolean
      creditText: { setText: ReturnType<typeof vi.fn> }
      freeSpinText: { setText: ReturnType<typeof vi.fn> }
      betText: { setText: ReturnType<typeof vi.fn> }
      setSpinButtonEnabled: ReturnType<typeof vi.fn>
      refreshWalletHud: () => void
      adjustBet: (direction: -1 | 1) => void
    }
    state.selectedBetCredits = '50'
    state.walletDefaultBetCredits = '10'
    state.betInitialized = true
    state.creditText = { setText: vi.fn() }
    state.freeSpinText = { setText: vi.fn() }
    state.betText = { setText: vi.fn() }
    state.setSpinButtonEnabled = vi.fn()

    state.refreshWalletHud()
    state.adjustBet(1)

    expect(state.selectedBetCredits).toBe('5')
    expect(state.betText.setText).toHaveBeenLastCalledWith('下注 5')
  })

  it('allows paid bets up to 10000 credits and stops there', () => {
    const scene = new LuckyScene({
      ...hooks,
      getSlotState: () => ({
        credits: '50000', freeSpinsRemaining: 0, betCredits: '10',
        loyaltyEnabled: true, loyaltyAvailable: true,
      }),
    })
    const state = scene as unknown as {
      selectedBetCredits: string
      refreshBetHud: ReturnType<typeof vi.fn>
      adjustBet: (direction: -1 | 1) => void
    }
    state.selectedBetCredits = '5000'
    state.refreshBetHud = vi.fn()

    state.adjustBet(1)
    expect(state.selectedBetCredits).toBe('10000')
    state.adjustBet(1)
    expect(state.selectedBetCredits).toBe('10000')
  })

  it('normalizes and plays every returned bonus spin in order', async () => {
    const scene = new LuckyScene(hooks)
    const playBonusSpin = vi.fn(async () => undefined)
    const setVisible = vi.fn()
    const round = {
      token: 'bonus-token',
      offer: {
        token: 'bonus-token', kind: 'free_spins' as const, expiresAt: '', betCredits: '10',
        bonusCount: 4, bonusMultiplier: 2,
        options: [{ choice: 0, spins: 8, multiplier: 5 }],
      },
      generation: 7,
      container: { setVisible },
      message: { setText: vi.fn() },
      choices: [],
      returnButton: { setVisible: vi.fn() },
    }
    const state = scene as unknown as {
      bonusRound: typeof round
      playBonusSpin: typeof playBonusSpin
      playBonusSpins: (
        result: import('../types').SlotBonusServerResult,
        value: typeof round,
      ) => Promise<void>
    }
    state.bonusRound = round
    state.playBonusSpin = playBonusSpin
    const result: import('../types').SlotBonusServerResult = {
      round_id: 'bonus-round', kind: 'free_spins', choice: 0, reward_credits: '30',
      credits_after: '130', claimed_at: '2026-07-29T05:00:00Z', spins: 2,
      multiplier: 5, bonus_multiplier: 2, total_base_payout: '3',
      spin_results: [
        { grid: normalizeServerGrid([]), total_payout: 1 },
        { grid: normalizeServerGrid([]), payout_credits: '2' },
      ],
    }

    await state.playBonusSpins(result, round)

    expect(setVisible).toHaveBeenCalledWith(false)
    expect(playBonusSpin).toHaveBeenCalledTimes(2)
    expect(playBonusSpin.mock.calls[0]?.[0].payout_credits).toBe('1')
    expect(playBonusSpin.mock.calls[1]?.[0].payout_credits).toBe('2')
    expect(playBonusSpin.mock.calls.map((call) => call[1])).toEqual([1, 2])
  })

  it('queues the next FIFO bonus until the current total reward has been shown', () => {
    const nextOffer = {
      token: 'next-token', kind: 'free_spins' as const, expiresAt: '', betCredits: '10',
      bonusCount: 3, bonusMultiplier: 1,
      options: [{ choice: 0, spins: 8, multiplier: 5 }],
    }
    const scene = new LuckyScene(hooks)
    const closeBonusRound = vi.fn()
    const state = scene as unknown as {
      bonusRound: { token: string }
      bonusClaiming: boolean
      queuedBonusOffer: typeof nextOffer | null
      closeBonusRound: typeof closeBonusRound
      syncPendingBonus: (slotState: import('../types').SlotSceneState) => void
    }
    state.bonusRound = { token: 'current-token' }
    state.bonusClaiming = true
    state.queuedBonusOffer = null
    state.closeBonusRound = closeBonusRound

    state.syncPendingBonus({
      credits: '100', freeSpinsRemaining: 0, betCredits: '10',
      loyaltyEnabled: true, loyaltyAvailable: true, pendingSlotBonus: nextOffer,
    })

    expect(state.queuedBonusOffer).toEqual(nextOffer)
    expect(closeBonusRound).not.toHaveBeenCalled()
  })
})
