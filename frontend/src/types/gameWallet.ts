/** Server-authoritative game loyalty wallet (R21). Credits are integer strings. */

export interface GameWallet {
  user_id: number
  account_balance: string
  credits: string
  free_spins_remaining: number
  loyalty_enabled: boolean
  loyalty_available: boolean
  checkin_credits: string
  slot_bet_credits: string
  daily_reward_limit: number
  checked_in_today: boolean
  today_checkin_at?: string | null
  local_date: string
  /** Legacy R20 fields; always false/zero in R21. */
  exchange_enabled?: boolean
  exchange_available?: boolean
  exchange_rate?: string
  daily_limit?: string
  daily_exchanged?: string
  daily_remaining?: string | null
  pending_slot_bonus?: GameWalletSlotBonusRound
  wallet_created_at: string
  wallet_updated_at: string
}

export interface GameWalletCheckinResult {
  id: string
  user_id: number
  local_date: string
  credits_awarded: string
  credits_before: string
  credits_after: string
  idempotency_key: string
  idempotent_replay?: boolean
  created_at: string
}

export interface GameCreditGiftRequest {
  recipient_email: string
  credits: string
}

export interface GameCreditGiftResult {
  id: string
  sender_user_id: number
  recipient_user_id: number
  recipient_email: string
  credits: string
  sender_credits_before: string
  sender_credits_after: string
  recipient_credits_before: string
  recipient_credits_after: string
  idempotency_key: string
  idempotent_replay?: boolean
  created_at: string
}

export interface GameWalletSpinRequest {
  bet_credits?: string
}

export type GameFruitDoor = 'bar' | '77' | 'star' | 'watermelon' | 'bell' | 'mango' | 'orange' | 'apple'

export type GameFruitBets = Record<GameFruitDoor, string>

export interface GameFruitRewardStop {
	index: number
	symbol: GameFruitDoor
	size: 'big' | 'small'
	multiplier: string
}

export interface GameFruitOutcome {
	kind: 'fruit' | 'send_light' | 'little_mary' | 'jackpot'
	symbol?: GameFruitDoor
	size?: 'big' | 'small'
	multiplier?: string
	extra_stops?: GameFruitRewardStop[]
	little_mary_stops?: GameFruitRewardStop[]
	jackpot_tier?: 'bronze' | 'silver' | 'gold'
	jackpot_multiplier?: string
}

export interface GameFruitSpinResult {
	id: string
	user_id: number
	bets: GameFruitBets
	total_bet: string
	stop_index: number
	outcome: GameFruitOutcome
	payout: string
	credits_before: string
	credits_after: string
	paytable_version: string
	idempotency_key: string
	idempotent_replay?: boolean
	created_at: string
}

export interface GameWalletSlotBonusRound {
  round_id: string
  token: string
  expires_at: string
  choices: number
  kind: 'free_spins'
  bet_credits: string
  bonus_count: number
  bonus_multiplier: number
  options: GameWalletSlotBonusOption[]
  status?: string
}

export interface GameWalletSlotBonusOption {
  choice: number
  spins: number
  multiplier: number
}

export interface GameWalletSlotBonusClaimRequest {
  token: string
  choice: number
}

export interface GameWalletSlotBonusClaimResult {
  round_id: string
  kind: 'free_spins'
  choice: number
  reward_credits: string
  credits_after: string
  claimed_at: string
  idempotent_replay?: boolean
  spins: number
  multiplier: number
  bonus_multiplier: number
  total_base_payout: string
  spin_results: GameWalletSlotBonusSpinResult[]
}

export interface GameWalletWinningLine {
  line_index: number
  symbol: string
  count: number
  multiplier: string
  payout: string
  /** [reel, row] pairs */
  positions: Array<[number, number]>
}

export interface GameWalletSpinResult {
  id: string
  user_id: number
  bet_credits: string
  payout_credits: string
  used_free_spin: boolean
  /** 3 rows x 5 reels of symbol codes */
  grid: string[][]
  stops: number[]
  winning_lines: GameWalletWinningLine[]
  scatter_count: number
  /** R23 BONUS count; absent on legacy R22 payloads. */
  bonus_count?: number
  free_spins_awarded: number
  free_spins_remaining: number
  credits_before: string
  credits_after: string
  paytable_version: string
  reel_strip_version: string
  idempotency_key: string
  idempotent_replay?: boolean
  created_at: string
  bonus_round?: GameWalletSlotBonusRound
}

export type GameWalletSlotBonusSpinResult = Partial<Omit<GameWalletSpinResult, 'grid' | 'winning_lines'>> & {
  grid: string[][]
  winning_lines?: Array<Omit<GameWalletWinningLine, 'multiplier' | 'payout'> & {
    multiplier: string | number
    payout: string | number
  }>
  total_payout?: string | number
}

export interface GameLeaderboardEntry {
  rank: number
  user_display: string
  score: string
  achieved_at: string
  is_current_user: boolean
}

export interface GameLeaderboardResult {
  game_id: string
  local_date: string
  items: GameLeaderboardEntry[]
  my_entry?: GameLeaderboardEntry | null
}

export interface GameLeaderboardScoreResult {
  game_id: string
  score: string
}

export interface GameLeaderboardQuery {
  game_id: string
  limit?: number
}

export interface GameWalletRewardItem {
  id: string
  title: string
  credit_cost: string
  voucher_value: string
  daily_stock: number
  expiry_days: number
  remaining_stock: number | null
  claimed_today: number
}

export interface GameWalletRewardList {
  items: GameWalletRewardItem[]
  daily_reward_limit: number
  claimed_today: number
  loyalty_enabled: boolean
  loyalty_available: boolean
}

export interface GameWalletRewardClaimRequest {
  reward_id: string
}

export interface GameWalletRewardClaimResult {
  id: string
  user_id: number
  reward_id: string
  title: string
  credit_cost: string
  voucher_value: string
  redeem_code: string
  claim_local_date: string
  credits_before: string
  credits_after: string
  expires_at?: string | null
  idempotency_key: string
  idempotent_replay?: boolean
  created_at: string
}

/** Ledger entry types from the server. */
export type GameWalletEntryType =
  | 'checkin'
  | 'slot_bet'
  | 'slot_payout'
  | 'slot_bonus'
	| 'fruit_bet'
	| 'fruit_payout'
	| 'gift_sent'
	| 'gift_received'
  | 'reward_claim'
  | string

export interface GameWalletTransaction {
  id: string
  user_id: number
  entry_type: GameWalletEntryType
  amount: string
  credits_before: string
  credits_after: string
  reference_type?: string
  reference_id?: string | null
  idempotency_key: string
  metadata?: unknown
  created_at: string
}

export interface GameWalletTransactionList {
  items: GameWalletTransaction[]
  next_before_id: string | null
}

export interface GameWalletTransactionQuery {
  limit?: number
  before_id?: string
}

/** Admin catalog row before serialization to exact JSON string. */
export interface GameLoyaltyRewardCatalogItem {
  id: string
  title: string
  credit_cost: string
  voucher_value: string
  daily_stock: number
  expiry_days: number
  enabled: boolean
  titles?: Record<string, string>
}
