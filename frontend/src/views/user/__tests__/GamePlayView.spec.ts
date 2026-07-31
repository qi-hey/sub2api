import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import GamePlayView from '../GamePlayView.vue'

const {
  routeState,
  claimGameWalletSlotBonus,
  createGameWalletIdempotencyKey,
  getGameWallet,
  readBestScore,
  saveBestScore,
  spinGameWalletFruit,
  spinGameWalletSlot,
  submitGameLeaderboardScore,
  showError,
} = vi.hoisted(() => ({
  routeState: { id: 'tetris' },
  claimGameWalletSlotBonus: vi.fn(),
  createGameWalletIdempotencyKey: vi.fn(),
  getGameWallet: vi.fn(),
  readBestScore: vi.fn(),
  saveBestScore: vi.fn(),
  spinGameWalletFruit: vi.fn(),
  spinGameWalletSlot: vi.fn(),
  submitGameLeaderboardScore: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: routeState }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@/api/gameWallet', () => ({
  claimGameWalletSlotBonus,
  createGameWalletIdempotencyKey,
  getGameWallet,
  spinGameWalletFruit,
  spinGameWalletSlot,
  submitGameLeaderboardScore,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh-CN' },
      t: (key: string) => {
        const messages: Record<string, string> = {
          'gameCenter.games.tetris.controls': '使用方向键移动，按 P 暂停。',
          'gameCenter.slot.failed': '旋转失败，请稍后重试。',
          'gameCenter.wallet.loadFailed': '游戏积分加载失败。',
        }
        return messages[key] ?? key
      },
    }),
  }
})

vi.mock('@/features/game-center/storage', () => ({
  readBestScore,
  saveBestScore,
}))

const GameCanvasStub = defineComponent({
  name: 'GameCanvasStub',
  props: {
    gameId: { type: String, required: true },
    label: { type: String, required: true },
    controlsDescription: { type: String, required: true },
    getSlotState: { type: Function, default: undefined },
    requestSlotSpin: { type: Function, default: undefined },
    requestSlotBonus: { type: Function, default: undefined },
    getFruitState: { type: Function, default: undefined },
    requestFruitSpin: { type: Function, default: undefined },
    formatSlotError: { type: Function, default: undefined },
    prefersReducedMotion: { type: Function, required: true },
  },
  emits: [
    'score', 'status', 'slot-result', 'slot-error', 'slot-bonus-result', 'slot-bonus-error',
    'fruit-result', 'fruit-error',
  ],
  setup(_props, { expose }) {
    expose({
      syncSlotWallet: vi.fn(),
      syncWalletState: vi.fn(),
      sendControl: vi.fn(),
      focus: vi.fn(),
    })
    return {}
  },
  template: '<div data-testid="game-canvas" :data-controls-description="controlsDescription"></div>',
})

const wallet = {
  user_id: 7,
  account_balance: '42.5',
  credits: '900',
  free_spins_remaining: 2,
  loyalty_enabled: true,
  loyalty_available: true,
  checkin_credits: '50',
  slot_bet_credits: '10',
  daily_reward_limit: 5,
  checked_in_today: false,
  local_date: '2026-07-28',
  wallet_created_at: '2026-07-28T00:00:00Z',
  wallet_updated_at: '2026-07-28T00:00:00Z',
}

const bonusOptions = [
  { choice: 0, spins: 8, multiplier: 5 },
  { choice: 1, spins: 12, multiplier: 3 },
  { choice: 2, spins: 20, multiplier: 2 },
]

const spinResult = {
  id: 'spin-1',
  user_id: 7,
  bet_credits: '10',
  payout_credits: '360',
  used_free_spin: false,
  grid: [
    ['JP', 'DR', 'IG', 'JA', 'BE'],
    ['WILD', 'WILD', 'WILD', 'SCATTER', 'BE'],
    ['DR', 'IG', 'JA', 'BE', 'JP'],
  ],
  stops: [1, 2, 3, 4, 5],
  winning_lines: [{
    line_index: 0,
    symbol: 'WILD',
    count: 3,
    multiplier: '36',
    payout: '360',
    positions: [[0, 1], [1, 1], [2, 1]],
  }],
  scatter_count: 1,
  bonus_count: 3,
  free_spins_awarded: 2,
  free_spins_remaining: 4,
  credits_before: '900',
  credits_after: '1250',
  paytable_version: 'v1',
  reel_strip_version: 'v1',
  idempotency_key: '00000000-0000-4000-8000-000000000001',
  created_at: '2026-07-28T03:00:00Z',
  bonus_round: {
    round_id: 'bonus-1',
    token: 'server-bonus-token',
    expires_at: '2026-07-28T03:05:00Z',
    choices: 1,
    kind: 'free_spins' as const,
    bet_credits: '10',
    bonus_count: 4,
    bonus_multiplier: 2,
    options: bonusOptions,
  },
}

const bonusResult = {
  round_id: 'bonus-1',
  kind: 'free_spins' as const,
  choice: 1,
  reward_credits: '66',
  credits_after: '1316',
  claimed_at: '2026-07-28T03:01:00Z',
  spins: 12,
  multiplier: 3,
  bonus_multiplier: 2,
  total_base_payout: '11',
  spin_results: [],
}

const fruitBets = {
  bar: '1', '77': '0', star: '0', watermelon: '0',
  bell: '0', mango: '0', orange: '0', apple: '0',
}

const fruitResult = {
  id: 'fruit-1',
  bets: fruitBets,
  total_bet: '1',
  stop_index: 0,
  outcome: { kind: 'fruit' as const, symbol: 'bar' as const, size: 'big' as const, multiplier: '100' },
  payout: '100',
  credits_before: '900',
  credits_after: '999',
  paytable_version: 'fruit-v1',
  idempotency_key: '00000000-0000-4000-8000-000000000099',
  created_at: '2026-07-29T00:00:00Z',
}

function mountView(gameId: string) {
  routeState.id = gameId
  return mount(GamePlayView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        GameCanvas: GameCanvasStub,
        Icon: true,
      },
    },
  })
}

describe('GamePlayView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    routeState.id = 'tetris'
    createGameWalletIdempotencyKey.mockReturnValue('00000000-0000-4000-8000-000000000099')
    readBestScore.mockReturnValue(0)
    saveBestScore.mockImplementation((_gameId: string, nextScore: number) => nextScore)
    getGameWallet.mockResolvedValue({ ...wallet })
    spinGameWalletFruit.mockResolvedValue({ ...fruitResult })
    spinGameWalletSlot.mockResolvedValue({ ...spinResult })
    claimGameWalletSlotBonus.mockResolvedValue({ ...bonusResult })
    submitGameLeaderboardScore.mockResolvedValue({ game_id: 'tetris', score: '100' })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('passes the selected local game copy without calling wallet APIs', async () => {
    const wrapper = mountView('tetris')
    await flushPromises()

    const canvas = wrapper.getComponent(GameCanvasStub)
    expect(canvas.attributes('data-controls-description')).toBe('使用方向键移动，按 P 暂停。')
    expect(canvas.props('getSlotState')).toBeUndefined()
    expect(canvas.props('requestSlotSpin')).toBeUndefined()
    expect(getGameWallet).not.toHaveBeenCalled()
    expect(spinGameWalletSlot).not.toHaveBeenCalled()
  })

  it('injects the server wallet and returns the authoritative slot grid', async () => {
    const wrapper = mountView('lucky')
    await flushPromises()

    const canvas = wrapper.getComponent(GameCanvasStub)
    const getSlotState = canvas.props('getSlotState') as () => Record<string, unknown>
    const requestSlotSpin = canvas.props('requestSlotSpin') as (bet: string) => Promise<typeof spinResult>

    expect(getGameWallet).toHaveBeenCalledTimes(1)
    expect(getSlotState()).toEqual({
      credits: '900',
      freeSpinsRemaining: 2,
      betCredits: '10',
      loyaltyEnabled: true,
      loyaltyAvailable: true,
    })

    await expect(requestSlotSpin('20')).resolves.toEqual(spinResult)
    expect(spinGameWalletSlot).toHaveBeenCalledWith(
      {},
      '00000000-0000-4000-8000-000000000099',
    )
    expect(spinResult.grid[1]).toEqual(['WILD', 'WILD', 'WILD', 'SCATTER', 'BE'])
    expect(spinResult.bonus_round.kind).toBe('free_spins')

    canvas.vm.$emit('slot-result', spinResult)
    await flushPromises()
    const scores = wrapper.findAll('.game-player__scoreboard strong')
    expect(scores[0]?.text()).toBe('1,250')
    expect(scores[1]?.text()).toBe('4')
  })

  it('renders the independent 243-ways rules panel only beside the slot game', async () => {
    const slotWrapper = mountView('lucky')
    await flushPromises()
    expect(slotWrapper.find('[data-testid="slot-rules"]').exists()).toBe(true)
    expect(slotWrapper.find('.slot-rules__ways').text()).toContain('243')

    const localGameWrapper = mountView('tetris')
    expect(localGameWrapper.find('[data-testid="slot-rules"]').exists()).toBe(false)
  })

  it('passes a selected paid bet to the slot endpoint', async () => {
    getGameWallet.mockResolvedValue({ ...wallet, free_spins_remaining: 0 })
    const wrapper = mountView('lucky')
    await flushPromises()
    const requestSlotSpin = wrapper.getComponent(GameCanvasStub)
      .props('requestSlotSpin') as (bet: string) => Promise<typeof spinResult>

    await requestSlotSpin('50')

    expect(spinGameWalletSlot).toHaveBeenCalledWith(
      { bet_credits: '50' },
      '00000000-0000-4000-8000-000000000099',
    )
  })

  it('requires an uncertain fruit spin to retry with the original bets and idempotency key', async () => {
    spinGameWalletFruit
      .mockRejectedValueOnce({ status: 0, message: 'response lost' })
      .mockResolvedValueOnce({ ...fruitResult })
    const wrapper = mountView('merge2048')
    await flushPromises()
    const requestFruitSpin = wrapper.getComponent(GameCanvasStub)
      .props('requestFruitSpin') as (bets: typeof fruitBets) => Promise<typeof fruitResult>

    await expect(requestFruitSpin(fruitBets)).rejects.toMatchObject({ status: 0 })
    const changedBets = { ...fruitBets, bar: '0', apple: '1' }
    await expect(requestFruitSpin(changedBets)).rejects.toMatchObject({
      code: 'GAME_FRUIT_RETRY_BETS_MISMATCH',
      fruitRetryBets: fruitBets,
    })
    expect(spinGameWalletFruit).toHaveBeenCalledTimes(1)

    await expect(requestFruitSpin(fruitBets)).resolves.toEqual(fruitResult)
    expect(spinGameWalletFruit).toHaveBeenNthCalledWith(
      2,
      fruitBets,
      '00000000-0000-4000-8000-000000000099',
    )
    expect(createGameWalletIdempotencyKey).toHaveBeenCalledTimes(1)
  })

  it('collapses concurrent spin requests into one upstream call', async () => {
    let resolveSpin: ((result: typeof spinResult) => void) | undefined
    spinGameWalletSlot.mockReturnValue(new Promise((resolve) => { resolveSpin = resolve }))
    const wrapper = mountView('lucky')
    await flushPromises()

    const requestSlotSpin = wrapper.getComponent(GameCanvasStub)
      .props('requestSlotSpin') as (bet: string) => Promise<typeof spinResult>
    const first = requestSlotSpin('20')
    const second = requestSlotSpin('20')

    expect(spinGameWalletSlot).toHaveBeenCalledTimes(1)
    resolveSpin?.(spinResult)
    await expect(Promise.all([first, second])).resolves.toEqual([spinResult, spinResult])
  })

  it('reuses the spin idempotency key after an uncertain network failure', async () => {
    spinGameWalletSlot
      .mockRejectedValueOnce({ status: 0, message: 'response lost' })
      .mockResolvedValueOnce({ ...spinResult })
    const wrapper = mountView('lucky')
    await flushPromises()
    const requestSlotSpin = wrapper.getComponent(GameCanvasStub)
      .props('requestSlotSpin') as (bet: string) => Promise<typeof spinResult>

    await expect(requestSlotSpin('20')).rejects.toMatchObject({ status: 0 })
    await expect(requestSlotSpin('10')).resolves.toEqual(spinResult)

    expect(createGameWalletIdempotencyKey).toHaveBeenCalledTimes(1)
    expect(spinGameWalletSlot).toHaveBeenNthCalledWith(
      1,
      {},
      '00000000-0000-4000-8000-000000000099',
    )
    expect(spinGameWalletSlot).toHaveBeenNthCalledWith(
      2,
      {},
      '00000000-0000-4000-8000-000000000099',
    )
  })

  it('shows a Chinese message when a slot request fails', async () => {
    const wrapper = mountView('lucky')
    await flushPromises()

    wrapper.getComponent(GameCanvasStub).vm.$emit('slot-error', new Error('upstream failed'))
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('旋转失败，请稍后重试。')
  })

  it('prevents a paid spin when credits are below the configured bet', async () => {
    getGameWallet.mockResolvedValue({
      ...wallet,
      credits: '9',
      free_spins_remaining: 0,
      slot_bet_credits: '10',
    })
    const wrapper = mountView('lucky')
    await flushPromises()
    const requestSlotSpin = wrapper.getComponent(GameCanvasStub)
      .props('requestSlotSpin') as (bet: string) => Promise<typeof spinResult>

    await expect(requestSlotSpin('10')).rejects.toMatchObject({
      code: 'GAME_LOYALTY_INSUFFICIENT_CREDITS',
    })
    expect(spinGameWalletSlot).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('积分不足，无法旋转，请先签到获取积分')
  })

  it('uses the explicit insufficient-credits copy for a backend business error', async () => {
    const wrapper = mountView('lucky')
    await flushPromises()

    wrapper.getComponent(GameCanvasStub).vm.$emit('slot-error', {
      reason: 'GAME_LOYALTY_INSUFFICIENT_CREDITS',
    })
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('积分不足，无法旋转，请先签到获取积分')
  })

  it('collapses concurrent bonus claims and applies only the server reward', async () => {
    let resolveBonus: ((result: typeof bonusResult) => void) | undefined
    claimGameWalletSlotBonus.mockReturnValue(new Promise((resolve) => { resolveBonus = resolve }))
    getGameWallet
      .mockResolvedValueOnce({ ...wallet })
      .mockResolvedValueOnce({
        ...wallet,
        credits: '1316',
        pending_slot_bonus: {
          round_id: 'bonus-next', token: 'next-server-token', expires_at: '2026-07-28T03:10:00Z',
          choices: 3, kind: 'free_spins' as const, bet_credits: '10', bonus_count: 3,
          bonus_multiplier: 1, options: bonusOptions, status: 'pending',
        },
      })
    const wrapper = mountView('lucky')
    await flushPromises()
    const canvas = wrapper.getComponent(GameCanvasStub)
    const requestSlotBonus = canvas.props('requestSlotBonus') as (
      token: string,
      choice: number,
    ) => Promise<typeof bonusResult>

    const first = requestSlotBonus('secret-token', 1)
    const second = requestSlotBonus('secret-token', 1)
    expect(claimGameWalletSlotBonus).toHaveBeenCalledTimes(1)
    expect(claimGameWalletSlotBonus).toHaveBeenCalledWith(
      { token: 'secret-token', choice: 1 },
      '00000000-0000-4000-8000-000000000099',
    )

    resolveBonus?.(bonusResult)
    await expect(Promise.all([first, second])).resolves.toEqual([bonusResult, bonusResult])
    expect(claimGameWalletSlotBonus).toHaveBeenCalledTimes(1)
    canvas.vm.$emit('slot-bonus-result', bonusResult)
    await flushPromises()
    expect(wrapper.findAll('.game-player__scoreboard strong')[0]?.text()).toBe('1,316')
    expect(getGameWallet).toHaveBeenCalledTimes(2)
    const getSlotState = canvas.props('getSlotState') as () => Record<string, unknown>
    expect(getSlotState()).toMatchObject({
      pendingSlotBonus: { token: 'next-server-token', kind: 'free_spins' },
    })
  })

  it('reuses a bonus claim idempotency key until a definite outcome arrives', async () => {
    claimGameWalletSlotBonus
      .mockRejectedValueOnce({ status: 0, message: 'response lost' })
      .mockResolvedValueOnce({ ...bonusResult })
    const wrapper = mountView('lucky')
    await flushPromises()
    const requestSlotBonus = wrapper.getComponent(GameCanvasStub).props('requestSlotBonus') as (
      token: string,
      choice: number,
    ) => Promise<typeof bonusResult>

    await expect(requestSlotBonus('secret-token', 1)).rejects.toMatchObject({ status: 0 })
    await expect(requestSlotBonus('secret-token', 1)).resolves.toEqual(bonusResult)

    expect(createGameWalletIdempotencyKey).toHaveBeenCalledTimes(1)
    expect(claimGameWalletSlotBonus).toHaveBeenNthCalledWith(
      1,
      { token: 'secret-token', choice: 1 },
      '00000000-0000-4000-8000-000000000099',
    )
    expect(claimGameWalletSlotBonus).toHaveBeenNthCalledWith(
      2,
      { token: 'secret-token', choice: 1 },
      '00000000-0000-4000-8000-000000000099',
    )
  })

  it('submits the all-time high score without resetting on the next day', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 6, 28, 10, 0, 0))
    readBestScore.mockReturnValue(999999)
    const wrapper = mountView('tetris')
    const canvas = wrapper.getComponent(GameCanvasStub)

    canvas.vm.$emit('score', 100)
    canvas.vm.$emit('status', 'complete')
    await flushPromises()
    canvas.vm.$emit('score', 90)
    canvas.vm.$emit('status', 'complete')
    await flushPromises()

    expect(submitGameLeaderboardScore).toHaveBeenCalledTimes(1)
    expect(submitGameLeaderboardScore).toHaveBeenLastCalledWith('tetris', 999999)

    vi.setSystemTime(new Date(2026, 6, 29, 10, 0, 0))
    canvas.vm.$emit('score', 90)
    canvas.vm.$emit('status', 'complete')
    await flushPromises()

    expect(submitGameLeaderboardScore).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('keeps a failed leaderboard score pending for a later retry', async () => {
    submitGameLeaderboardScore
      .mockRejectedValueOnce({ status: 503 })
      .mockResolvedValueOnce({ game_id: 'tetris', score: '321' })
    const wrapper = mountView('tetris')
    const canvas = wrapper.getComponent(GameCanvasStub)

    canvas.vm.$emit('score', 321)
    canvas.vm.$emit('status', 'game-over')
    await flushPromises()
    canvas.vm.$emit('status', 'game-over')
    await flushPromises()

    expect(submitGameLeaderboardScore).toHaveBeenCalledTimes(2)
    expect(submitGameLeaderboardScore).toHaveBeenNthCalledWith(1, 'tetris', 321)
    expect(submitGameLeaderboardScore).toHaveBeenNthCalledWith(2, 'tetris', 321)
    wrapper.unmount()
  })

  it('flushes a queued leaderboard score while leaving the view', async () => {
    const wrapper = mountView('tetris')
    wrapper.getComponent(GameCanvasStub).vm.$emit('score', 456)

    wrapper.unmount()
    await flushPromises()

    expect(submitGameLeaderboardScore).toHaveBeenCalledWith('tetris', 456)
  })

  it('never submits lucky scores to the local-game leaderboard endpoint', async () => {
    const wrapper = mountView('lucky')
    await flushPromises()
    const canvas = wrapper.getComponent(GameCanvasStub)

    canvas.vm.$emit('score', 999999)
    canvas.vm.$emit('status', 'complete')
    await flushPromises()

    expect(submitGameLeaderboardScore).not.toHaveBeenCalled()
  })

  it('passes a pending BONUS from the wallet snapshot back into the slot scene', async () => {
    getGameWallet.mockResolvedValue({
      ...wallet,
      pending_slot_bonus: {
        round_id: 'bonus-restored',
        token: 'restored-server-token',
        expires_at: '2026-07-29T03:10:00Z',
        choices: 3,
        kind: 'free_spins',
        bet_credits: '10',
        bonus_count: 5,
        bonus_multiplier: 4,
        options: bonusOptions,
        status: 'pending',
      },
    })
    const wrapper = mountView('lucky')
    await flushPromises()

    const getSlotState = wrapper.getComponent(GameCanvasStub)
      .props('getSlotState') as () => Record<string, unknown>
    expect(getSlotState()).toMatchObject({
      pendingSlotBonus: {
        token: 'restored-server-token',
        kind: 'free_spins',
        expiresAt: '2026-07-29T03:10:00Z',
        betCredits: '10',
        bonusCount: 5,
        bonusMultiplier: 4,
        options: bonusOptions,
      },
    })
  })

  it('keeps the server pending BONUS token after a failed claim refresh', async () => {
    const pendingWallet = {
      ...wallet,
      pending_slot_bonus: {
        round_id: 'bonus-pending',
        token: 'retry-server-token',
        expires_at: '2026-07-29T03:10:00Z',
        choices: 3,
        kind: 'free_spins' as const,
        bet_credits: '10',
        bonus_count: 4,
        bonus_multiplier: 2,
        options: bonusOptions,
        status: 'pending',
      },
    }
    getGameWallet
      .mockResolvedValueOnce({ ...wallet })
      .mockResolvedValueOnce(pendingWallet)
    const wrapper = mountView('lucky')
    await flushPromises()
    const canvas = wrapper.getComponent(GameCanvasStub)

    canvas.vm.$emit('slot-bonus-error', { status: 503 })
    await flushPromises()

    expect(getGameWallet).toHaveBeenCalledTimes(2)
    const getSlotState = canvas.props('getSlotState') as () => Record<string, unknown>
    expect(getSlotState()).toMatchObject({
      pendingSlotBonus: {
        token: 'retry-server-token',
        kind: 'free_spins',
      },
    })
  })
})
