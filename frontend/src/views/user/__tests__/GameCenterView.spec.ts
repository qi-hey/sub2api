import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import GameCenterView from '../GameCenterView.vue'

const {
  checkInGameWallet,
  claimGameWalletReward,
  createGameWalletIdempotencyKey,
  giftGameWalletCredits,
  getGameLeaderboard,
  getGameWallet,
  getGameWalletRewards,
  getGameWalletTransactions,
  readBestScore,
  refreshUser,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  checkInGameWallet: vi.fn(),
  claimGameWalletReward: vi.fn(),
  createGameWalletIdempotencyKey: vi.fn(() => '00000000-0000-4000-8000-000000000001'),
  giftGameWalletCredits: vi.fn(),
  getGameLeaderboard: vi.fn(),
  getGameWallet: vi.fn(),
  getGameWalletRewards: vi.fn(),
  getGameWalletTransactions: vi.fn(),
  readBestScore: vi.fn(() => 0),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/gameWallet', () => ({
  checkInGameWallet,
  claimGameWalletReward,
  createGameWalletIdempotencyKey,
  giftGameWalletCredits,
  getGameLeaderboard,
  getGameWallet,
  getGameWalletRewards,
  getGameWalletTransactions,
}))

vi.mock('@/features/game-center/storage', () => ({
  readBestScore,
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({ refreshUser }),
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorCode: (error: unknown) => {
    if (!error || typeof error !== 'object') return undefined
    return (error as { reason?: string }).reason
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh-CN' },
      t: (key: string, params?: Record<string, string>) => {
        const messages: Record<string, string> = {
          'gameCenter.checkin.success': '签到成功，获得 {credits} 积分',
          'gameCenter.wallet.checkinCredits': '签到可得 {credits} 积分',
          'gameCenter.quickLinks.profile': '个人资料',
          'gameCenter.quickLinks.checkin': '每日签到',
          'gameCenter.quickLinks.gift': '赠送积分',
          'gameCenter.quickLinks.ledger': '积分明细',
          'gameCenter.quickLinks.redeem': '使用兑换码',
          'gameCenter.leaderboard.me': '我',
          'gameCenter.leaderboard.myRank': '我的名次',
          'gameCenter.leaderboard.loadFailed': '排行榜加载失败，请稍后重试。',
          'gameCenter.rewards.dailyLimit': '今日全局限额：{used}/{limit}',
          'gameCenter.rewards.expiryDays': '有效期 {days} 天',
          'gameCenter.rewards.success': '兑换成功，请妥善保存兑换码。',
          'gameCenter.rewards.copied': '兑换码已复制',
          'gameCenter.gift.confirmation': '确认向 {email} 赠送 {credits} 积分？赠送成功后不能撤回。',
          'gameCenter.gift.success': '已向 {email} 赠送 {credits} 积分',
          'gameCenter.gift.failed': '积分赠送失败，请稍后重试。',
          'gameCenter.ledger.types.checkin': '签到',
          'gameCenter.ledger.types.slot_bet': '下注',
          'gameCenter.ledger.types.slot_payout': '中奖',
          'gameCenter.ledger.types.gift_sent': '赠送积分',
          'gameCenter.ledger.types.gift_received': '收到赠送',
          'gameCenter.ledger.types.reward_claim': '奖励兑换',
          'gameCenter.ledger.types.unknown': '其他',
          'gameCenter.wallet.maintenance': '游戏积分与奖励暂未开放，请稍后再试。其他本地游戏仍可畅玩，但不会增加可兑换积分。',
        }
        return (messages[key] ?? key).replace(
          /\{(\w+)\}/g,
          (_, token) => params?.[token] ?? `{${token}}`,
        )
      },
    }),
  }
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

const rewards = {
  items: [{
    id: 'api-1',
    title: '体验额度',
    credit_cost: '500',
    voucher_value: '1.25',
    daily_stock: 3,
    expiry_days: 7,
    remaining_stock: 2,
    claimed_today: 0,
  }],
  daily_reward_limit: 5,
  claimed_today: 1,
  loyalty_enabled: true,
  loyalty_available: true,
}

const transactions = {
  items: [{
    id: '11',
    user_id: 7,
    entry_type: 'checkin',
    amount: '50',
    credits_before: '850',
    credits_after: '900',
    idempotency_key: 'k1',
    created_at: '2026-07-28T01:00:00Z',
  }],
  next_before_id: '11',
}

const leaderboard = {
  game_id: 'lucky',
  local_date: '2026-07-28',
  items: [
    {
      rank: 1,
      user_display: 'Alice',
      score: '12000',
      achieved_at: '2026-07-28T05:00:00Z',
      is_current_user: false,
    },
    {
      rank: 2,
      user_display: '我自己',
      score: '9800',
      achieved_at: '2026-07-28T04:00:00Z',
      is_current_user: true,
    },
  ],
  my_entry: {
    rank: 2,
    user_display: '我自己',
    score: '9800',
    achieved_at: '2026-07-28T04:00:00Z',
    is_current_user: true,
  },
}

function mountView() {
  return mount(GameCenterView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        GameCardArt: true,
        Icon: true,
        RouterLink: {
          props: ['to'],
          template: '<a :href="to"><slot /></a>',
        },
      },
    },
  })
}

describe('GameCenterView loyalty wallet', () => {
  beforeEach(() => {
    vi.clearAllMocks()
	createGameWalletIdempotencyKey.mockReset()
		createGameWalletIdempotencyKey.mockReturnValue('00000000-0000-4000-8000-000000000001')
		checkInGameWallet.mockReset()
		claimGameWalletReward.mockReset()
		giftGameWalletCredits.mockReset()
    getGameWallet.mockResolvedValue({ ...wallet })
    getGameLeaderboard.mockResolvedValue({
      ...leaderboard,
      items: leaderboard.items.map((item) => ({ ...item })),
      my_entry: { ...leaderboard.my_entry },
    })
    getGameWalletRewards.mockResolvedValue({ ...rewards, items: [...rewards.items] })
    getGameWalletTransactions.mockResolvedValue({
      items: [...transactions.items],
      next_before_id: transactions.next_before_id,
    })
    checkInGameWallet.mockResolvedValue({
      id: '1',
      user_id: 7,
      local_date: '2026-07-28',
      credits_awarded: '50',
      credits_before: '900',
      credits_after: '950',
      idempotency_key: '00000000-0000-4000-8000-000000000001',
      created_at: '2026-07-28T02:00:00Z',
    })
    claimGameWalletReward.mockResolvedValue({
      id: '2',
      user_id: 7,
      reward_id: 'api-1',
      title: '体验额度',
      credit_cost: '500',
      voucher_value: '1.25',
      redeem_code: 'GAME-TOKEN-XYZ',
      claim_local_date: '2026-07-28',
      credits_before: '950',
      credits_after: '450',
      idempotency_key: '00000000-0000-4000-8000-000000000001',
      created_at: '2026-07-28T03:00:00Z',
    })
		giftGameWalletCredits.mockResolvedValue({
			id: 'gift-1',
			sender_user_id: 7,
			recipient_user_id: 8,
			recipient_email: 'friend@example.com',
			credits: '25',
			sender_credits_before: '900',
			sender_credits_after: '875',
			recipient_credits_before: '100',
			recipient_credits_after: '125',
			idempotency_key: '00000000-0000-4000-8000-000000000001',
			created_at: '2026-07-28T03:30:00Z',
		})
    refreshUser.mockResolvedValue(undefined)
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    })
  })

  it('renders loyalty stats and hides the old exchange UI', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="game-wallet-credits"]').text()).toBe('900')
    expect(wrapper.get('[data-testid="game-wallet-balance"]').text()).toBe('US$42.50')
    expect(wrapper.get('[data-testid="game-wallet-free-spins"]').text()).toBe('2')
    expect(wrapper.get('[data-testid="game-wallet-checkin-status"]').text()).toContain(
      'gameCenter.wallet.checkinPending',
    )
    expect(wrapper.find('[data-testid="game-exchange-submit"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="reward-item-api-1"]').text()).toContain('体验额度')
    expect(wrapper.get('[data-testid="ledger-item-11"]').text()).toContain('签到')
  })

  it('puts the game catalog before wallet features and renders compact quick links', async () => {
    const wrapper = mountView()
    await flushPromises()

    const catalog = wrapper.get('.arcade-catalog').element
    const walletPanel = wrapper.get('.wallet-panel').element
    expect(catalog.compareDocumentPosition(walletPanel) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

    const quickLinks = wrapper.get('[data-testid="game-quick-links"]')
    expect(quickLinks.find('a[href="/profile"]').text()).toBe('个人资料')
    expect(quickLinks.find('a[href="#game-checkin"]').text()).toBe('每日签到')
    expect(quickLinks.find('a[href="#game-gift"]').text()).toBe('赠送积分')
    expect(quickLinks.find('a[href="#game-ledger"]').text()).toBe('积分明细')
    expect(quickLinks.find('a[href="/redeem"]').text()).toBe('使用兑换码')
    expect(wrapper.get('#game-checkin').classes()).toContain('anchor-target')
    expect(wrapper.get('#game-gift').classes()).toContain('anchor-target')
    expect(wrapper.get('#game-ledger').classes()).toContain('anchor-target')
  })

	it('previews and confirms a gift by email, then refreshes wallet history', async () => {
		const wrapper = mountView()
		await flushPromises()

		await wrapper.get('[data-testid="game-credit-gift-email"]').setValue(' Friend@Example.com ')
		await wrapper.get('[data-testid="game-credit-gift-credits"]').setValue('25')
		await wrapper.get('[data-testid="game-credit-gift-prepare"]').trigger('submit')
		await wrapper.vm.$nextTick()

		expect(wrapper.get('.gift-confirmation').text()).toContain('friend@example.com')
		expect(wrapper.get('.gift-confirmation').text()).toContain('25')
		await wrapper.get('[data-testid="game-credit-gift-submit"]').trigger('click')
		await flushPromises()

		expect(giftGameWalletCredits).toHaveBeenCalledWith(
			{ recipient_email: 'friend@example.com', credits: '25' },
			'00000000-0000-4000-8000-000000000001',
		)
		expect(wrapper.get('[data-testid="game-wallet-credits"]').text()).toBe('875')
		expect(wrapper.get('[data-testid="game-credit-gift-success"]').text()).toContain(
			'已向 friend@example.com 赠送 25 积分',
		)
		expect(getGameWalletTransactions).toHaveBeenCalledTimes(2)
	})

	it('blocks duplicate gift submits and reuses the idempotency key after an uncertain failure', async () => {
		let finish: ((value: unknown) => void) | undefined
		giftGameWalletCredits.mockReturnValueOnce(new Promise((resolve) => { finish = resolve }))
		const wrapper = mountView()
		await flushPromises()
		await wrapper.get('[data-testid="game-credit-gift-email"]').setValue('friend@example.com')
		await wrapper.get('[data-testid="game-credit-gift-credits"]').setValue('25')
		await wrapper.get('[data-testid="game-credit-gift-prepare"]').trigger('submit')
		await wrapper.get('[data-testid="game-credit-gift-submit"]').trigger('click')
		await wrapper.get('[data-testid="game-credit-gift-submit"]').trigger('click')
		expect(giftGameWalletCredits).toHaveBeenCalledTimes(1)
		finish?.({
			id: 'gift-1', sender_user_id: 7, recipient_user_id: 8,
			recipient_email: 'friend@example.com', credits: '25',
			sender_credits_before: '900', sender_credits_after: '875',
			recipient_credits_before: '100', recipient_credits_after: '125',
			idempotency_key: '00000000-0000-4000-8000-000000000001',
			created_at: '2026-07-28T03:30:00Z',
		})
		await flushPromises()
		wrapper.unmount()

		giftGameWalletCredits.mockReset()
		giftGameWalletCredits
			.mockRejectedValueOnce(new Error('connection lost'))
			.mockResolvedValueOnce({
				id: 'gift-2', sender_user_id: 7, recipient_user_id: 8,
				recipient_email: 'friend@example.com', credits: '25',
				sender_credits_before: '900', sender_credits_after: '875',
				recipient_credits_before: '100', recipient_credits_after: '125',
				idempotency_key: '00000000-0000-4000-8000-000000000001',
				created_at: '2026-07-28T03:31:00Z',
			})
		const retryWrapper = mountView()
		await flushPromises()
		await retryWrapper.get('[data-testid="game-credit-gift-email"]').setValue('friend@example.com')
		await retryWrapper.get('[data-testid="game-credit-gift-credits"]').setValue('25')
		await retryWrapper.get('[data-testid="game-credit-gift-prepare"]').trigger('submit')
		await retryWrapper.get('[data-testid="game-credit-gift-submit"]').trigger('click')
		await flushPromises()
		await retryWrapper.get('[data-testid="game-credit-gift-submit"]').trigger('click')
		await flushPromises()

		const keys = giftGameWalletCredits.mock.calls.map((call) => call[1])
		expect(keys).toEqual([
			'00000000-0000-4000-8000-000000000001',
			'00000000-0000-4000-8000-000000000001',
		])
	})

  it('loads the slot leaderboard first, exposes all game tabs, and highlights the current user', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getGameLeaderboard).toHaveBeenCalledWith({ game_id: 'lucky', limit: 10 })
    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs).toHaveLength(7)
    expect(tabs[0].attributes('data-testid')).toBe('leaderboard-tab-lucky')
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    expect(wrapper.get('[data-testid="leaderboard-row-2"]').classes()).toContain('leaderboard-row--self')
    expect(wrapper.get('[data-testid="leaderboard-row-1"]').text()).toContain('12,000')
    expect(wrapper.find('[data-testid="leaderboard-my-entry"]').exists()).toBe(false)
  })

  it('loads a new leaderboard on tab change and shows the current user outside the top ten', async () => {
    getGameLeaderboard
      .mockResolvedValueOnce({ ...leaderboard })
      .mockResolvedValueOnce({
        game_id: 'snake',
        local_date: '2026-07-28',
        items: [{ ...leaderboard.items[0] }],
        my_entry: {
          rank: 21,
          user_display: '我自己',
          score: '640',
          achieved_at: '2026-07-28T03:00:00Z',
          is_current_user: true,
        },
      })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="leaderboard-tab-snake"]').trigger('click')
    await flushPromises()

    expect(getGameLeaderboard).toHaveBeenLastCalledWith({ game_id: 'snake', limit: 10 })
    expect(wrapper.get('[data-testid="leaderboard-my-entry"]').text()).toContain('#21')
    expect(wrapper.get('[data-testid="leaderboard-my-entry"]').text()).toContain('640')
  })

  it('shows leaderboard errors and retries the active game', async () => {
    getGameLeaderboard
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({ ...leaderboard })

    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('排行榜加载失败')

    await wrapper.get('[data-testid="leaderboard-retry"]').trigger('click')
    await flushPromises()
    expect(getGameLeaderboard).toHaveBeenLastCalledWith({ game_id: 'lucky', limit: 10 })
    expect(wrapper.find('[data-testid="leaderboard-retry"]').exists()).toBe(false)
  })

  it('shows leaderboard loading and empty states', async () => {
    getGameLeaderboard.mockReturnValueOnce(new Promise(() => {}))
    const loadingWrapper = mountView()
    await loadingWrapper.vm.$nextTick()
    expect(loadingWrapper.find('[data-testid="leaderboard-loading"]').exists()).toBe(true)
    loadingWrapper.unmount()

    getGameLeaderboard.mockResolvedValueOnce({
      game_id: 'lucky',
      local_date: '2026-07-28',
      items: [],
      my_entry: null,
    })
    const emptyWrapper = mountView()
    await flushPromises()
    expect(emptyWrapper.find('[data-testid="leaderboard-empty"]').exists()).toBe(true)
  })

  it('check-in reuses the same idempotency key on uncertain retries and blocks double submit', async () => {
    let finish: ((value: unknown) => void) | undefined
    checkInGameWallet.mockReturnValueOnce(new Promise((resolve) => { finish = resolve }))
    checkInGameWallet.mockResolvedValueOnce({
      id: '1',
      credits_awarded: '50',
      credits_after: '950',
      created_at: '2026-07-28T02:00:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    const button = wrapper.get('[data-testid="game-checkin-submit"]')
    await button.trigger('click')
    await button.trigger('click')
    expect(checkInGameWallet).toHaveBeenCalledTimes(1)
    expect((button.element as HTMLButtonElement).disabled).toBe(true)

    finish?.({
      id: '1',
      credits_awarded: '50',
      credits_after: '950',
      created_at: '2026-07-28T02:00:00Z',
    })
    await flushPromises()

	wrapper.unmount()
	checkInGameWallet.mockReset()
	checkInGameWallet.mockClear()
	createGameWalletIdempotencyKey.mockReset()
	createGameWalletIdempotencyKey.mockReturnValue('00000000-0000-4000-8000-000000000099')
    checkInGameWallet
      .mockRejectedValueOnce(new Error('lost'))
      .mockResolvedValueOnce({
        id: '9',
        credits_awarded: '50',
        credits_after: '950',
        created_at: '2026-07-28T04:00:00Z',
      })

	const retryWrapper = mountView()
	await flushPromises()
	await retryWrapper.get('[data-testid="game-checkin-submit"]').trigger('click')
    await flushPromises()
	await retryWrapper.get('[data-testid="game-checkin-submit"]').trigger('click')
    await flushPromises()

    const keys = checkInGameWallet.mock.calls.map((call) => call[0])
    expect(keys[keys.length - 2]).toBe(keys[keys.length - 1])
	expect(retryWrapper.get('[data-testid="checkin-success"]').text()).toContain('50')
  })

  it('claims a reward, shows the redeem code, and copies it', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="reward-claim-api-1"]').trigger('click')
    await flushPromises()

    expect(claimGameWalletReward).toHaveBeenCalledWith(
      { reward_id: 'api-1' },
      '00000000-0000-4000-8000-000000000001',
    )
    expect(wrapper.get('[data-testid="reward-redeem-code"]').text()).toBe('GAME-TOKEN-XYZ')
    await wrapper.get('[data-testid="reward-copy-code"]').trigger('click')
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('GAME-TOKEN-XYZ')
    expect(showSuccess).toHaveBeenCalledWith('兑换码已复制')
  })

  it('shows maintenance state when loyalty is unavailable and still lists games', async () => {
    getGameWallet.mockResolvedValueOnce({
      ...wallet,
      loyalty_enabled: false,
      loyalty_available: false,
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="loyalty-unavailable"]').text()).toContain('暂未开放')
    expect(
      (wrapper.get('[data-testid="game-checkin-submit"]').element as HTMLButtonElement).disabled,
    ).toBe(true)
    expect(wrapper.findAll('[data-game-id]').length).toBeGreaterThanOrEqual(7)
  })

  it('loads more ledger entries with the cursor', async () => {
    getGameWalletTransactions
      .mockResolvedValueOnce({
        items: [...transactions.items],
        next_before_id: '11',
      })
      .mockResolvedValueOnce({
        items: [{
          id: '10',
          user_id: 7,
          entry_type: 'slot_bet',
          amount: '-10',
          credits_before: '860',
          credits_after: '850',
          idempotency_key: 'k2',
          created_at: '2026-07-28T00:30:00Z',
        }],
        next_before_id: null,
      })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="ledger-load-more"]').trigger('click')
    await flushPromises()

    expect(getGameWalletTransactions).toHaveBeenLastCalledWith({ limit: 20, before_id: '11' })
    expect(wrapper.get('[data-testid="ledger-item-10"]').text()).toContain('下注')
  })
})
