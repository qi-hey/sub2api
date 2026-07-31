/** Server symbol codes and Chinese display labels for the 5×3 Macau-style slot. */

export const SLOT_REEL_COUNT = 5
export const SLOT_ROW_COUNT = 3

export const SLOT_SYMBOL_CODES = ['JP', 'DR', 'IG', 'JA', 'BE', 'WILD', 'SCATTER', 'BONUS'] as const

export type SlotSymbolCode = (typeof SLOT_SYMBOL_CODES)[number]

/** Simplified Chinese labels shown on reels and a11y text. */
export const SLOT_SYMBOL_LABELS: Record<SlotSymbolCode, string> = {
  JP: '金色 7',
  DR: '龙',
  IG: '元宝',
  JA: '玉牌',
  BE: '金铃',
  WILD: '百搭',
  SCATTER: '福星',
  BONUS: '彩金',
}

/** Accent colors for generated symbol tiles (black/red/gold cabinet). */
export const SLOT_SYMBOL_COLORS: Record<SlotSymbolCode, number> = {
  JP: 0xfbbf24,
  DR: 0xdc2626,
  IG: 0xf59e0b,
  JA: 0x34d399,
  BE: 0xfcd34d,
  WILD: 0xf472b6,
  SCATTER: 0x60a5fa,
  BONUS: 0xa855f7,
}

/** Direct credits for one symbol's longest continuous result at a 10-credit bet. */
export const SLOT_LINE_PAYTABLE = {
  JP: { 3: 80, 4: 200, 5: 400 },
  DR: { 3: 40, 4: 80, 5: 160 },
  IG: { 3: 10, 4: 30, 5: 54 },
  JA: { 3: 5, 4: 20, 5: 30 },
  BE: { 3: 5, 4: 10, 5: 18 },
} as const

export const SLOT_SCATTER_FREE_SPINS = { 3: 5, 4: 8, 5: 12 } as const
export const SLOT_SCATTER_PAYTABLE = { 3: 10, 4: 30, 5: 100 } as const
export const SLOT_BIG_WIN_MULTIPLIER = 25n
export const SLOT_JACKPOT_WIN_MULTIPLIER = 60n

export function isSlotSymbolCode(value: string): value is SlotSymbolCode {
  return (SLOT_SYMBOL_CODES as readonly string[]).includes(value)
}

export function slotSymbolLabel(code: string): string {
  return isSlotSymbolCode(code) ? SLOT_SYMBOL_LABELS[code] : code
}

/** Ensure a 3×5 grid of known codes; unknown cells fall back to 金铃. */
export function normalizeServerGrid(grid: string[][] | null | undefined): string[][] {
  const rows: string[][] = []
  for (let row = 0; row < SLOT_ROW_COUNT; row += 1) {
    const sourceRow = Array.isArray(grid?.[row]) ? grid![row]! : []
    const next: string[] = []
    for (let reel = 0; reel < SLOT_REEL_COUNT; reel += 1) {
      const raw = sourceRow[reel]
      next.push(typeof raw === 'string' && isSlotSymbolCode(raw) ? raw : 'BE')
    }
    rows.push(next)
  }
  return rows
}

/** Integer-string compare without floats: payout is a "big win" when ≥ 25× bet. */
export function isBigWinPayout(payoutCredits: string, betCredits: string): boolean {
  const payout = parseCreditsInteger(payoutCredits)
  const bet = parseCreditsInteger(betCredits)
  if (payout === null || bet === null || bet <= 0n) return false
  return payout >= bet * SLOT_BIG_WIN_MULTIPLIER
}

export function hasPositivePayout(payoutCredits: string): boolean {
  const payout = parseCreditsInteger(payoutCredits)
  return payout !== null && payout > 0n
}

export function parseCreditsInteger(value: string): bigint | null {
  const trimmed = value.trim()
  if (!/^-?\d+$/.test(trimmed)) return null
  try {
    return BigInt(trimmed)
  } catch {
    return null
  }
}

/** Format integer credit strings with grouping; never routes through float. */
export function formatCreditsDisplay(value: string): string {
  const trimmed = value.trim()
  if (!/^-?\d+$/.test(trimmed)) return trimmed
  const negative = trimmed.startsWith('-')
  const digits = negative ? trimmed.slice(1) : trimmed
  const grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return negative ? `-${grouped}` : grouped
}

export function randomSlotSymbol(exclude?: string): SlotSymbolCode {
  const pool = exclude
    ? SLOT_SYMBOL_CODES.filter((code) => code !== exclude)
    : [...SLOT_SYMBOL_CODES]
  const index = Math.floor(Math.random() * pool.length)
  return pool[index] ?? 'BE'
}

export interface SlotSpinPresentation {
  grid: string[][]
  payoutCredits: string
  betCredits: string
  freeSpinsAwarded: number
  freeSpinsRemaining: number
  creditsAfter: string
  winningLineCount: number
  usedFreeSpin: boolean
}

export function toSpinPresentation(result: {
  grid: string[][]
  payout_credits: string
  bet_credits: string
  free_spins_awarded: number
  free_spins_remaining: number
  credits_after: string
  winning_lines: unknown[]
  used_free_spin: boolean
}): SlotSpinPresentation {
  return {
    grid: normalizeServerGrid(result.grid),
    payoutCredits: result.payout_credits,
    betCredits: result.bet_credits,
    freeSpinsAwarded: result.free_spins_awarded,
    freeSpinsRemaining: result.free_spins_remaining,
    creditsAfter: result.credits_after,
    winningLineCount: Array.isArray(result.winning_lines) ? result.winning_lines.length : 0,
    usedFreeSpin: result.used_free_spin,
  }
}
