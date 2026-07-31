import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('../client', () => ({
  apiClient: { get, post },
}))

import {
  checkInGameWallet,
  claimGameWalletSlotBonus,
  claimGameWalletReward,
  createGameWalletIdempotencyKey,
  getGameWallet,
  getGameLeaderboard,
  getGameWalletRewards,
  getGameWalletTransactions,
	giftGameWalletCredits,
  spinGameWalletSlot,
  spinGameWalletFruit,
  submitGameLeaderboardScore,
} from '../gameWallet'

describe('game wallet API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads the wallet, rewards, and transactions from user endpoints', async () => {
    const wallet = {
      user_id: 7,
      account_balance: '12.5',
      credits: '800',
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
    const rewards = {
      items: [{
        id: 'api-1',
        title: '体验额度',
        credit_cost: '500',
        voucher_value: '1.25',
        daily_stock: 3,
        expiry_days: 7,
        remaining_stock: 2,
        claimed_today: 1,
      }],
      daily_reward_limit: 5,
      claimed_today: 1,
      loyalty_enabled: true,
      loyalty_available: true,
    }
    const transactions = {
      items: [{
        id: '1',
        user_id: 7,
        entry_type: 'checkin' as const,
        amount: '50',
        credits_before: '750',
        credits_after: '800',
        idempotency_key: 'wallet-request-1',
        created_at: '2026-07-28T00:00:00Z',
      }],
      next_before_id: null,
    }
    get
      .mockResolvedValueOnce({ data: wallet })
      .mockResolvedValueOnce({ data: rewards })
      .mockResolvedValueOnce({ data: transactions })

    await expect(getGameWallet()).resolves.toEqual(wallet)
    await expect(getGameWalletRewards()).resolves.toEqual(rewards)
    await expect(getGameWalletTransactions()).resolves.toEqual(transactions)
    expect(get).toHaveBeenNthCalledWith(1, '/user/game-wallet')
    expect(get).toHaveBeenNthCalledWith(2, '/user/game-wallet/rewards')
    expect(get).toHaveBeenNthCalledWith(3, '/user/game-wallet/transactions', { params: {} })
  })

  it('sends a UUID idempotency key with check-in', async () => {
    const response = { id: '1', credits_awarded: '50' }
    post.mockResolvedValue({ data: response })

    await expect(checkInGameWallet()).resolves.toEqual(response)

    const config = post.mock.calls[0]?.[2]
    expect(post).toHaveBeenCalledWith(
      '/user/game-wallet/check-in',
      {},
      expect.objectContaining({
        headers: expect.objectContaining({
          'Idempotency-Key': expect.any(String),
        }),
      }),
    )
    expect(config.headers['Idempotency-Key']).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    )
  })

  it('sends spin and claim requests with caller-provided idempotency keys', async () => {
    post
      .mockResolvedValueOnce({ data: { id: 'spin-1', grid: [['JP']] } })
      .mockResolvedValueOnce({ data: { id: 'claim-1', redeem_code: 'CODE' } })

    await spinGameWalletSlot({ bet_credits: '50' }, '00000000-0000-4000-8000-000000000001')
    await claimGameWalletReward(
      { reward_id: 'api-1' },
      '00000000-0000-4000-8000-000000000002',
    )

    expect(post.mock.calls[0]?.[0]).toBe('/user/game-wallet/slot/spin')
    expect(post.mock.calls[0]?.[1]).toEqual({ bet_credits: '50' })
    expect(post.mock.calls[0]?.[2]?.headers).toEqual({
      'Idempotency-Key': '00000000-0000-4000-8000-000000000001',
    })
    expect(post.mock.calls[1]?.[0]).toBe('/user/game-wallet/rewards/claim')
    expect(post.mock.calls[1]?.[1]).toEqual({ reward_id: 'api-1' })
    expect(post.mock.calls[1]?.[2]?.headers).toEqual({
      'Idempotency-Key': '00000000-0000-4000-8000-000000000002',
    })
    expect(createGameWalletIdempotencyKey()).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    )
  })

	it('gifts credits by recipient email with a caller-provided idempotency key', async () => {
		const result = {
			id: 'gift-1', recipient_email: 'friend@example.com', credits: '25',
			sender_credits_after: '775',
		}
		post.mockResolvedValueOnce({ data: result })

		await expect(giftGameWalletCredits(
			{ recipient_email: 'friend@example.com', credits: '25' },
			'00000000-0000-4000-8000-000000000005',
		)).resolves.toEqual(result)
		expect(post).toHaveBeenCalledWith(
			'/user/game-wallet/gifts',
			{ recipient_email: 'friend@example.com', credits: '25' },
			{ headers: { 'Idempotency-Key': '00000000-0000-4000-8000-000000000005' } },
		)
	})

  it('omits bet_credits when consuming a server-side free spin', async () => {
    post.mockResolvedValueOnce({ data: { id: 'free-spin-1', used_free_spin: true } })

    await spinGameWalletSlot({}, '00000000-0000-4000-8000-000000000003')

    expect(post).toHaveBeenCalledWith(
      '/user/game-wallet/slot/spin',
      {},
      { headers: { 'Idempotency-Key': '00000000-0000-4000-8000-000000000003' } },
    )
  })

  it('calls bonus and leaderboard endpoints with their exact contracts', async () => {
    const bonus = {
      round_id: 'bonus-1',
      kind: 'free_spins',
      choice: 2,
      reward_credits: '88',
      credits_after: '988',
      claimed_at: '2026-07-28T04:00:00Z',
      spins: 20,
      multiplier: 2,
      bonus_multiplier: 4,
      total_base_payout: '11',
      spin_results: [{ grid: [['JP']], total_payout: '5' }],
    }
    const leaderboard = { game_id: 'tetris', local_date: '2026-07-28', items: [], my_entry: null }
    const scoreResult = { game_id: 'tetris', score: '2048' }
    post.mockResolvedValueOnce({ data: bonus }).mockResolvedValueOnce({ data: scoreResult })
    get.mockResolvedValueOnce({ data: leaderboard })

    await expect(claimGameWalletSlotBonus(
      { token: 'secret-token', choice: 2 },
      '00000000-0000-4000-8000-000000000003',
    )).resolves.toEqual(bonus)
    await expect(getGameLeaderboard({ game_id: 'tetris', limit: 10 })).resolves.toEqual(leaderboard)
    await expect(submitGameLeaderboardScore('tetris', 2048)).resolves.toEqual(scoreResult)

    expect(post).toHaveBeenNthCalledWith(
      1,
      '/user/game-wallet/slot/bonus/claim',
      { token: 'secret-token', choice: 2 },
      { headers: { 'Idempotency-Key': '00000000-0000-4000-8000-000000000003' } },
    )
    expect(get).toHaveBeenCalledWith('/user/game-wallet/leaderboard', {
      params: { game_id: 'tetris', limit: 10 },
    })
    expect(post).toHaveBeenNthCalledWith(
      2,
      '/user/game-wallet/leaderboard/score',
      { game_id: 'tetris', score: 2048 },
    )
  })

  it('submits fruit bets with an idempotency key', async () => {
    const result = { id: 'fruit-1', stop_index: 7, payout: '15', credits_after: '1015' }
    post.mockResolvedValueOnce({ data: result })
    const bets = {
      bar: '1', '77': '0', star: '0', watermelon: '0',
      bell: '0', mango: '0', orange: '0', apple: '2',
    } as const

    await expect(spinGameWalletFruit(bets, '00000000-0000-4000-8000-000000000004'))
      .resolves.toEqual(result)
    expect(post).toHaveBeenCalledWith(
      '/user/game-wallet/fruit/spin',
      { bets },
      { headers: { 'Idempotency-Key': '00000000-0000-4000-8000-000000000004' } },
    )
  })
})
