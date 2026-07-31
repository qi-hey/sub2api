export const GAME_IDS = [
  'snake',
  'tetris',
  'breakout',
  'merge2048',
  'orbital',
  'hoarder',
  'lucky',
] as const

export type GameId = (typeof GAME_IDS)[number]

export type GameControl =
  | 'left'
  | 'right'
  | 'up'
  | 'down'
  | 'rotate'
  | 'action'
  | 'drop'
  | 'pause'
  | 'restart'

export type GameStatus = 'ready' | 'playing' | 'paused' | 'game-over' | 'complete'

export interface GameCatalogItem {
  id: GameId
  accent: string
  accentSoft: string
  titleKey: string
  subtitleKey: string
  descriptionKey: string
  controlsKey: string
  mobileControls: GameControl[]
}

/** Live wallet snapshot for the server-authoritative slot cabinet. */
export interface SlotSceneState {
  credits: string
  freeSpinsRemaining: number
  betCredits: string
  loyaltyEnabled: boolean
  loyaltyAvailable: boolean
  pendingSlotBonus?: SlotSceneBonusOffer
}

export type SlotBonusRoundKind = 'free_spins'

export interface SlotBonusOption {
  choice: number
  spins: number
  multiplier: number
}

export interface SlotSceneBonusOffer {
  token: string
  kind: SlotBonusRoundKind
  expiresAt: string
  betCredits: string
  bonusCount: number
  bonusMultiplier: number
  options: SlotBonusOption[]
}

export type FruitDoor = 'bar' | '77' | 'star' | 'watermelon' | 'bell' | 'mango' | 'orange' | 'apple'
export type FruitBets = Record<FruitDoor, string>

export interface FruitSceneState {
	credits: string
	loyaltyEnabled: boolean
	loyaltyAvailable: boolean
}

export interface FruitRewardStop {
	index: number
	symbol: FruitDoor
	size: 'big' | 'small'
	multiplier: string
}

export interface FruitSpinServerResult {
	id: string
	bets: FruitBets
	total_bet: string
	stop_index: number
		outcome: {
			kind: 'fruit' | 'send_light' | 'little_mary' | 'jackpot'
			center_index?: number
			symbol?: FruitDoor
		size?: 'big' | 'small'
		multiplier?: string
		extra_stops?: FruitRewardStop[]
		little_mary_stops?: FruitRewardStop[]
		jackpot_tier?: 'bronze' | 'silver' | 'gold'
		jackpot_multiplier?: string
	}
	payout: string
	credits_before: string
	credits_after: string
	paytable_version: string
	idempotency_key: string
	idempotent_replay?: boolean
	created_at: string
}

export interface SlotBonusServerResult {
  round_id: string
  kind: SlotBonusRoundKind
  choice: number
  reward_credits: string
  credits_after: string
  claimed_at: string
  idempotent_replay?: boolean
  spins: number
  multiplier: number
  bonus_multiplier: number
  total_base_payout: string
  spin_results: SlotBonusSpinResult[]
}

export interface SlotWinningLine {
  line_index: number
  symbol: string
  count: number
  multiplier: string
  payout: string
  positions: Array<[number, number]>
}

/**
 * Authoritative spin payload the scene must land on.
 * Mirrors GameWalletSpinResult without importing API types into Phaser modules.
 */
export interface SlotSpinServerResult {
  id: string
  bet_credits: string
  payout_credits: string
  used_free_spin: boolean
  grid: string[][]
  stops: number[]
  winning_lines: SlotWinningLine[]
  scatter_count: number
	/** BONUS is independent from SCATTER: 1/2 modify the base-game payout; 3+ open a bonus round. */
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
  bonus_round?: {
    round_id: string
    token: string
    expires_at: string
    choices: number
    kind: SlotBonusRoundKind
    bet_credits: string
    bonus_count: number
    bonus_multiplier: number
    options: SlotBonusOption[]
    status?: string
  }
}

export type SlotBonusSpinResult = Partial<Omit<SlotSpinServerResult, 'grid' | 'winning_lines'>> & {
  grid: string[][]
  winning_lines?: Array<Omit<SlotWinningLine, 'multiplier' | 'payout'> & {
    multiplier: string | number
    payout: string | number
  }>
  /** Raw bonus plans may use the engine field before DTO normalization. */
  total_payout?: string | number
}

export interface SceneHooks {
  onScore: (score: number) => void
  onStatus: (status: GameStatus) => void
  /** Slot only: fetch live credits / free spins / bet for HUD. */
  getSlotState?: () => SlotSceneState
  /** Slot only: perform the server spin; scene never invents outcomes. */
  requestSlotSpin?: (betCredits: string) => Promise<SlotSpinServerResult>
  /** Slot only: notify host after a successful server spin is applied. */
  onSlotResult?: (result: SlotSpinServerResult) => void
  /** Slot only: notify host of spin transport / validation failures. */
  onSlotError?: (error: unknown) => void
  /** Slot only: claim one server-owned bonus choice. */
  requestSlotBonus?: (token: string, choice: number) => Promise<SlotBonusServerResult>
  /** Slot only: notify host after the bonus wallet result is applied. */
  onSlotBonusResult?: (result: SlotBonusServerResult) => void
  /** Slot only: notify host when a bonus claim fails. */
  onSlotBonusError?: (error: unknown) => void
  /** Slot only: format business errors for both the cabinet and host toast. */
  formatSlotError?: (error: unknown) => string
  /** Optional Chinese copy resolver (falls back to built-in zh strings). */
  t?: (key: string, params?: Record<string, string | number>) => string
  /** Respect prefers-reduced-motion without skipping the server request. */
  prefersReducedMotion?: () => boolean
	/** Fruit machine only: wallet snapshot and one authoritative settlement. */
	getFruitState?: () => FruitSceneState
	requestFruitSpin?: (bets: FruitBets) => Promise<FruitSpinServerResult>
	onFruitResult?: (result: FruitSpinServerResult) => void
	onFruitError?: (error: unknown) => void
	/** Fruit machine only: high-DPI backing scale while world coordinates stay at 900x620. */
	renderScale?: number
	/** Fruit machine only: returns the real media start clock so lamps stay on the soundtrack. */
	playSynchronizedSound?: (sound: 'fruit-center-spin' | 'fruit-outer-spin') => Promise<GameAudioSyncAnchor>
}

export interface GameAudioSyncAnchor {
	startedAtMs: number
	durationMs: number | null
}
