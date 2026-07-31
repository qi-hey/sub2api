import Phaser from 'phaser'
import { ArcadeScene, GAME_WIDTH } from '../ArcadeScene'
import {
  SLOT_REEL_COUNT,
  SLOT_ROW_COUNT,
  SLOT_SYMBOL_CODES,
  SLOT_SYMBOL_LABELS,
  SLOT_JACKPOT_WIN_MULTIPLIER,
  formatCreditsDisplay,
  hasPositivePayout,
  isBigWinPayout,
  normalizeServerGrid,
  randomSlotSymbol,
  slotSymbolLabel,
  type SlotSymbolCode,
} from '../../slotSymbols'
import type {
  GameControl,
  SceneHooks,
  SlotBonusOption,
  SlotBonusServerResult,
  SlotBonusSpinResult,
  SlotSceneBonusOffer,
  SlotSceneState,
  SlotSpinServerResult,
  SlotWinningLine,
} from '../../types'

const ACCENT_GOLD = 0xfbbf24
const ACCENT_RED = 0xdc2626
const PANEL_BLACK = 0x191022
const CABINET_RED = 0xb91c1c
const SCENE_HEIGHT = 600
const REEL_ORIGIN_X = 116
const REEL_ORIGIN_Y = 166
const REEL_CELL_WIDTH = 128
const REEL_CELL_HEIGHT = 76
const REEL_GAP_X = 8
const REEL_GAP_Y = 6
const INSUFFICIENT_CREDITS_MESSAGE = '积分不足，无法旋转，请先签到获取积分'
const COIN_TEXTURE_KEY = 'sub2api-lucky-coin-r25'
const SLOT_BET_OPTIONS = [
  '1', '2', '5', '10', '20', '50', '100',
  '200', '500', '1000', '2000', '5000', '10000',
] as const
const DEFAULT_BONUS_OPTIONS: SlotBonusOption[] = [
  { choice: 0, spins: 8, multiplier: 5 },
  { choice: 1, spins: 12, multiplier: 3 },
  { choice: 2, spins: 20, multiplier: 2 },
]
const BONUS_REEL_STOP_DELAY_MS = 170
const BONUS_RESULT_DISPLAY_MS = 750
const BASE_REEL_SPIN_MS = 800
const REEL_ANIMATION_TICK_MS = 16
const REEL_SCROLL_INTERVAL_MS = [42, 54, 68, 86, 108] as const
const REEL_STOP_DELAY_MS = [0, 250, 580, 980, 1480] as const
const AUTO_SPIN_DELAY_MS = 650

type WinCelebrationTier = 'regular' | 'big' | 'jackpot'

interface BonusChoiceView {
  container: Phaser.GameObjects.Container
  background: Phaser.GameObjects.Rectangle
  label: Phaser.GameObjects.Text
  option: SlotBonusOption
}

interface BonusRoundView {
  token: string
  offer: SlotSceneBonusOffer
  generation: number
  container: Phaser.GameObjects.Container
  message: Phaser.GameObjects.Text
  choices: BonusChoiceView[]
  returnButton: Phaser.GameObjects.Container
}

const DEFAULT_COPY: Record<string, string> = {
  'gameCenter.slot.title': '金运老虎机',
  'gameCenter.slot.spinning': '旋转中...',
  'gameCenter.slot.pressSpin': '好运就绪，点击旋转',
  'gameCenter.slot.win': '中奖 +{payout}',
  'gameCenter.slot.bigWin': '大奖！',
  'gameCenter.slot.noWin': '未中奖',
  'gameCenter.slot.jackpot': '头奖！',
  'gameCenter.slot.freeSpin': '免费旋转',
  'gameCenter.slot.freeSpinsAwarded': '获得 {count} 次免费旋转',
  'gameCenter.slot.bet': '下注 {credits}',
  'gameCenter.slot.betDecrease': '降低下注',
  'gameCenter.slot.betIncrease': '提高下注',
  'gameCenter.slot.credits': '积分 {credits}',
  'gameCenter.slot.freeSpinsLeft': '免费 {count}',
  'gameCenter.slot.spin': '旋转',
  'gameCenter.slot.autoSpin': '连续启动',
  'gameCenter.slot.autoSpinStop': '停止连转',
  'gameCenter.slot.insufficient': INSUFFICIENT_CREDITS_MESSAGE,
  'gameCenter.slot.maintenance': '积分玩法维护中，可欣赏机台但不会产生积分。',
  'gameCenter.slot.failed': '旋转失败，请稍后重试。',
  'gameCenter.slot.ways': '243 Ways',
  'gameCenter.slot.bonusTitle': '免费旋转奖励',
  'gameCenter.slot.bonusPrompt': '选择一个奖励方案',
  'gameCenter.slot.bonusMultiplier': '{count} 个 BONUS，最终奖励 ×{multiplier}',
  'gameCenter.slot.bonusOption': '{spins} 轮 ×{multiplier}',
  'gameCenter.slot.bonusWaiting': '正在生成奖励旋转...',
  'gameCenter.slot.bonusSpinProgress': '奖励旋转 {current}/{total}',
  'gameCenter.slot.bonusReward': '总奖励 +{credits} 积分（最终奖励 ×{multiplier}）',
  'gameCenter.slot.bonusCreateFailed': '奖励局创建失败，请重试',
  'gameCenter.slot.bonusRetry': '领取失败，请重试',
  'gameCenter.slot.bonusBack': '返回机台',
  'gameCenter.slot.winGroup': '{symbol}{count}轴',
  'gameCenter.slot.winSummary': '{details} = {score}分',
  'gameCenter.slot.winBonusSummary': '{details}；{count} 个彩金 ×{multiplier} = {score}分',
  'gameCenter.slot.symbols.JP': '金色 7',
  'gameCenter.slot.symbols.DR': '龙',
  'gameCenter.slot.symbols.IG': '元宝',
  'gameCenter.slot.symbols.JA': '玉牌',
  'gameCenter.slot.symbols.BE': '金铃',
}

const IDLE_GRID: string[][] = [
  ['JP', 'DR', 'IG', 'JA', 'BONUS'],
  ['WILD', 'SCATTER', 'JP', 'DR', 'IG'],
  ['JA', 'BE', 'BONUS', 'SCATTER', 'JP'],
]

function randomRollingReelSymbols(reel: number): SlotSymbolCode[] {
  const symbols: SlotSymbolCode[] = []
  for (let row = 0; row < SLOT_ROW_COUNT; row += 1) {
    const candidates = reel < 2
      ? SLOT_SYMBOL_CODES.filter((symbol) => !symbols.includes(symbol))
      : SLOT_SYMBOL_CODES
    const index = Math.floor(Math.random() * candidates.length)
    symbols.push(candidates[index] ?? randomSlotSymbol())
  }
  return symbols
}

function hasBonusOnFifthReel(grid: string[][]): boolean {
  return grid.some((row) => row[SLOT_REEL_COUNT - 1] === 'BONUS')
}

/**
 * Server-authoritative 5x3 / 243-ways Macau-style cabinet.
 * Animation may scroll locally, but final cells always equal the server grid.
 * Credits, free spins, and payouts are never invented client-side.
 */
export class LuckyScene extends ArcadeScene {
  private cellTexts: Phaser.GameObjects.Text[][] = []
  private cellBgs: Phaser.GameObjects.Rectangle[][] = []
  private cellIcons: Phaser.GameObjects.Graphics[][] = []
  private creditText!: Phaser.GameObjects.Text
  private freeSpinText!: Phaser.GameObjects.Text
  private betText!: Phaser.GameObjects.Text
  private messageText!: Phaser.GameObjects.Text
  private spinButtonLabel!: Phaser.GameObjects.Text
  private spinButton!: Phaser.GameObjects.Rectangle
  private autoSpinButton!: Phaser.GameObjects.Rectangle
  private autoSpinButtonLabel!: Phaser.GameObjects.Text
  private betDecreaseButton!: Phaser.GameObjects.Rectangle
  private betIncreaseButton!: Phaser.GameObjects.Rectangle
  private selectedBetCredits = '10'
  private walletDefaultBetCredits = '10'
  private betInitialized = false
  private lineHighlights: Phaser.GameObjects.GameObject[] = []
  private marqueeBulbs: Phaser.GameObjects.Arc[] = []
  private bonusRound: BonusRoundView | null = null
  private queuedBonusOffer: SlotSceneBonusOffer | null = null
  private bonusClaiming = false
  private bonusGeneration = 0
  private spinning = false
  private spinGeneration = 0
  private autoSpinEnabled = false
  private autoSpinGeneration = 0
  private lastAppliedGrid: string[][] = normalizeServerGrid(IDLE_GRID)
  private reducedMotion = false
  private celebrationObjects = new Set<Phaser.GameObjects.GameObject>()
  private celebrationGeneration = 0

  constructor(hooks: SceneHooks) {
    super('lucky', hooks)
  }

  create(): void {
    this.clearCelebration()
    this.scale.resize(GAME_WIDTH, SCENE_HEIGHT)
    this.spinning = false
    this.spinGeneration += 1
    this.autoSpinEnabled = false
    this.autoSpinGeneration += 1
    this.cellTexts = []
    this.cellBgs = []
    this.cellIcons = []
    this.lineHighlights = []
    this.marqueeBulbs = []
    this.bonusRound = null
    this.queuedBonusOffer = null
    this.bonusClaiming = false
    this.bonusGeneration += 1
    this.betInitialized = false
    this.reducedMotion = this.resolveReducedMotion()

    this.drawMacauCabinet()
    this.buildReelWindow()
    this.buildHud()
    this.applyGrid(this.lastAppliedGrid, false)
    this.refreshWalletHud()
    this.setMessage(this.copy('gameCenter.slot.pressSpin'))

    this.begin()
    // Local arcade score stays 0 — redeemable credits come only from the server HUD.
    this.setScore(0)
    this.bindControls((control) => this.handleControl(control))
    this.syncPendingBonus(this.readSlotState())
    this.events.once(Phaser.Scenes.Events.SHUTDOWN, () => this.clearCelebration())
    this.input.keyboard?.on('keydown-SPACE', () => {
      void this.spin()
    })
    this.input.keyboard?.on('keydown-P', () => this.handleControl('pause'))
    this.input.keyboard?.on('keydown-R', () => this.handleControl('restart'))
  }

  /** Host may call after wallet refresh without restarting the scene. */
  syncWalletState(): void {
    if (this.spinning) return
    this.refreshWalletHud()
    this.syncPendingBonus(this.readSlotState())
  }

  get isSpinning(): boolean {
    return this.spinning
  }

  /** Test helper: land a known server result without network. */
  applyServerResultForTests(result: SlotSpinServerResult): void {
    this.finishSpin(result, this.spinGeneration)
  }

  private handleControl(control: GameControl): void {
    if (this.handleCommonControl(control)) return
    if (control === 'action' || control === 'up') void this.spin()
  }

  protected canRestart(): boolean {
    return !this.spinning && !this.bonusClaiming
  }

  private async spin(): Promise<void> {
    if (this.status !== 'playing' || this.spinning || this.bonusRound) return

    const state = this.readSlotState()
    if (!state.loyaltyEnabled || !state.loyaltyAvailable) {
      this.stopAutoSpin()
      this.setMessage(this.copy('gameCenter.slot.maintenance'))
      return
    }

    const betCredits = state.freeSpinsRemaining > 0
      ? this.normalizeBetCredits(state.betCredits)
      : this.selectedBetCredits
    if (state.freeSpinsRemaining > 0 && this.selectedBetCredits !== betCredits) {
      this.selectedBetCredits = betCredits
      this.refreshBetHud(state)
    }

    if (!this.canAffordSpin(state, betCredits)) {
      const error = {
        code: 'GAME_LOYALTY_INSUFFICIENT_CREDITS',
        reason: 'GAME_LOYALTY_INSUFFICIENT_CREDITS',
        message: INSUFFICIENT_CREDITS_MESSAGE,
      }
      this.stopAutoSpin()
      this.setMessage(INSUFFICIENT_CREDITS_MESSAGE)
      this.hooksOnError(error)
      return
    }

    const requestSpin = this.hooksRequestSpin()
    if (!requestSpin) {
      this.stopAutoSpin()
      this.setMessage(this.copy('gameCenter.slot.maintenance'))
      return
    }

    this.spinning = true
    const generation = ++this.spinGeneration
    this.clearCelebration()
    this.clearLineHighlights()
    this.setSpinButtonEnabled(false)
    this.refreshBetHud(state)
    this.setMessage(this.copy('gameCenter.slot.spinning'))
    this.emitSound('slots-spin')

    // Kick off the server request immediately; animation runs in parallel.
    const resultPromise = requestSpin(betCredits)
    try {
      if (this.reducedMotion) {
        const result = await resultPromise
        if (generation !== this.spinGeneration) return
        this.finishSpin(result, generation)
        return
      }

      await this.animateReels(resultPromise, generation)
    } catch (error: unknown) {
      if (generation !== this.spinGeneration) return
      this.spinning = false
      this.stopAutoSpin()
      this.setSpinButtonEnabled(true)
      this.refreshBetHud(this.readSlotState())
      this.applyGrid(this.lastAppliedGrid, false)
      this.setMessage(this.formatSlotError(error))
      this.hooksOnError(error)
    }
  }

  private async animateReels(
    resultPromise: Promise<SlotSpinServerResult>,
    generation: number,
  ): Promise<void> {
    let settled: SlotSpinServerResult | null = null
    let rejected: unknown = null
    let resultReadyAt: number | null = null
    const startedAt = this.time.now
    void resultPromise.then(
      (value) => {
        settled = value
        resultReadyAt = this.time.now
      },
      (error: unknown) => { rejected = error },
    )

    const stoppedReels = new Set<number>()
    const lastScrollAt = Array<number>(SLOT_REEL_COUNT).fill(Number.NEGATIVE_INFINITY)

    await new Promise<void>((resolve, reject) => {
      const event = this.time.addEvent({
        delay: REEL_ANIMATION_TICK_MS,
        loop: true,
        callback: () => {
          if (generation !== this.spinGeneration) {
            event.remove(false)
            resolve()
            return
          }

          if (rejected) {
            event.remove(false)
            reject(rejected)
            return
          }

          const now = this.time.now
          let scrolled = false
          // The right-hand reels refresh less often, so the machine visibly
          // slows from left to right before each staggered stop.
          for (let reel = 0; reel < SLOT_REEL_COUNT; reel += 1) {
            if (stoppedReels.has(reel)) continue
            if (now-lastScrollAt[reel]! < REEL_SCROLL_INTERVAL_MS[reel]!) continue
            const symbols = randomRollingReelSymbols(reel)
            for (let row = 0; row < SLOT_ROW_COUNT; row += 1) {
              this.paintCell(row, reel, symbols[row]!, false)
            }
            lastScrollAt[reel] = now
            scrolled = true
          }
          if (scrolled) this.emitSound('slots-reel-tick')

          const readyAt = resultReadyAt
          if (settled === null || readyAt === null) return

          // Once the server result is ready, land the reels left to right with
          // increasingly longer gaps. Slow networks keep the reel motion going
          // without ever inventing a final grid.
          const nextReel = stoppedReels.size
          if (nextReel < SLOT_REEL_COUNT) {
            const stopStartAt = Math.max(startedAt + BASE_REEL_SPIN_MS, readyAt)
            if (now < stopStartAt + REEL_STOP_DELAY_MS[nextReel]!) return
            const grid = normalizeServerGrid(settled!.grid)
            for (let row = 0; row < SLOT_ROW_COUNT; row += 1) {
              this.paintCell(row, nextReel, grid[row]![nextReel]!, true)
            }
            stoppedReels.add(nextReel)
            this.emitSound('slots-reel-stop')
          }

          if (stoppedReels.size >= SLOT_REEL_COUNT) {
            event.remove(false)
            resolve()
          }
        },
      })
    })

    if (generation !== this.spinGeneration) return
    if (rejected) throw rejected
    if (!settled) {
      // Extremely slow network: wait for the promise without inventing a grid.
      settled = await resultPromise
    }
    this.finishSpin(settled, generation)
  }

  private finishSpin(result: SlotSpinServerResult, generation: number): void {
    if (generation !== this.spinGeneration) return

    const grid = normalizeServerGrid(result.grid)
    this.lastAppliedGrid = grid
    this.applyGrid(grid, true)
    this.highlightWinningLines(result)
    this.spinning = false
    const bonusCount = result.bonus_count ?? 0
    const fifthReelHasBonus = hasBonusOnFifthReel(grid)
    const bonusOffer = this.toSceneBonusOffer(result.bonus_round)
    const bonusCreationFailed = bonusCount >= 3 && !bonusOffer
    this.setSpinButtonEnabled(!bonusOffer)

    // HUD always mirrors server-owned balances — never a local tally.
    this.creditText.setText(
      this.copy('gameCenter.slot.credits', { credits: formatCreditsDisplay(result.credits_after) }),
    )
    this.freeSpinText.setText(
      this.copy('gameCenter.slot.freeSpinsLeft', { count: result.free_spins_remaining }),
    )
    this.selectedBetCredits = result.free_spins_remaining > 0
      ? this.walletDefaultBetCredits
      : this.normalizeBetCredits(result.bet_credits)
    this.refreshBetHud({
      ...this.readSlotState(),
      freeSpinsRemaining: result.free_spins_remaining,
    })

    const payout = result.payout_credits
    const winningSummary = this.formatWinningSummary(
      result.winning_lines,
      result.payout_credits,
      result.bonus_count ?? 0,
      fifthReelHasBonus,
    )
    const winMessage = winningSummary
      ?? this.copy('gameCenter.slot.win', { payout: formatCreditsDisplay(payout) })
    if (hasPositivePayout(payout)) {
      const celebrationTier = this.resolveWinCelebrationTier(payout, result.bet_credits)
      if (celebrationTier === 'jackpot') {
        this.setMessage(`${this.copy('gameCenter.slot.jackpot')} ${winMessage}`)
        this.emitSound('slots-jackpot')
        this.celebrateWin('jackpot')
      } else if (celebrationTier === 'big') {
        this.setMessage(`${this.copy('gameCenter.slot.bigWin')} ${winMessage}`)
        this.emitSound('slots-big-win')
        this.celebrateWin('big')
      } else {
        this.setMessage(winMessage)
        this.emitSound('slots-win')
        this.celebrateWin('regular')
      }
    } else {
      this.setMessage(this.copy('gameCenter.slot.noWin'))
      this.emitSound('slots-no-win')
    }

    if (result.free_spins_awarded > 0) {
      this.setMessage(
        `${this.messageText.text} · ${this.copy('gameCenter.slot.freeSpinsAwarded', {
          count: result.free_spins_awarded,
        })}`,
      )
      this.emitSound('slots-free-spins')
    }

    if (bonusCreationFailed) {
      this.setMessage(this.copy('gameCenter.slot.bonusCreateFailed'))
    }

    this.hooksOnResult(result)
    if (bonusCount >= 3) this.stopAutoSpin()
    if (bonusOffer) {
      this.enterBonusRound(bonusOffer)
      return
    }
    this.scheduleAutoSpin()
  }

  private applyGrid(grid: string[][], highlightStop: boolean): void {
    const normalized = normalizeServerGrid(grid)
    for (let row = 0; row < SLOT_ROW_COUNT; row += 1) {
      for (let reel = 0; reel < SLOT_REEL_COUNT; reel += 1) {
        this.paintCell(row, reel, normalized[row]![reel]!, highlightStop)
      }
    }
  }

  private paintCell(row: number, reel: number, code: string, emphasize: boolean): void {
    const text = this.cellTexts[row]?.[reel]
    const bg = this.cellBgs[row]?.[reel]
    const icon = this.cellIcons[row]?.[reel]
    if (!text || !bg || !icon) return
    const symbol = (code in SLOT_SYMBOL_LABELS ? code : 'BE') as SlotSymbolCode
    this.drawSlotSymbol(icon, text, symbol)
    bg.setFillStyle(emphasize ? 0xfff1b8 : 0xfff8df, 1)
    bg.setStrokeStyle(emphasize ? 3 : 1, emphasize ? ACCENT_RED : 0xd5a63a, emphasize ? 1 : 0.75)

    if (emphasize && !this.reducedMotion) {
      text.setScale(0.9)
      icon.setScale(0.9)
      this.tweens.add({ targets: [text, icon], scale: 1, duration: 150, ease: 'Back.easeOut' })
    }
  }

  private drawSlotSymbol(
    graphics: Phaser.GameObjects.Graphics,
    text: Phaser.GameObjects.Text,
    symbol: SlotSymbolCode,
  ): void {
    graphics.clear()
    text.setFontFamily('"Noto Sans SC", ui-sans-serif, sans-serif').setFontStyle('bold')
    if (symbol === 'JP') {
      graphics.fillStyle(0xdc2626, 1)
      graphics.fillCircle(0, 1, 31)
      graphics.lineStyle(4, 0xfbbf24, 1)
      graphics.strokeCircle(0, 1, 31)
      graphics.fillStyle(0xfff1a8, 0.9)
      graphics.fillCircle(-20, -18, 4)
      text.setText('7').setFontSize(48).setColor('#fff08a')
        .setStroke('#991b1b', 5)
      return
    }

    if (symbol === 'DR') {
      graphics.fillStyle(0xb91c1c, 1)
      graphics.fillRoundedRect(-38, -30, 76, 60, 18)
      graphics.lineStyle(3, 0xfbbf24, 1)
      graphics.strokeRoundedRect(-38, -30, 76, 60, 18)
      graphics.fillStyle(0xfef3c7, 0.9)
      graphics.fillCircle(-25, -18, 4)
      graphics.fillCircle(25, -18, 4)
      text.setText('龙').setFontSize(38).setColor('#fde68a')
        .setStroke('#7f1d1d', 4)
      return
    }

    if (symbol === 'IG') {
      graphics.fillStyle(0xf59e0b, 1)
      graphics.fillEllipse(0, 5, 82, 40)
      graphics.fillTriangle(-38, 4, -20, -27, -8, 8)
      graphics.fillTriangle(38, 4, 20, -27, 8, 8)
      graphics.lineStyle(3, 0x92400e, 0.85)
      graphics.strokeEllipse(0, 5, 82, 40)
      text.setText('元宝').setFontSize(20).setColor('#7c2d12')
      return
    }

    if (symbol === 'JA') {
      graphics.fillStyle(0x10b981, 1)
      graphics.fillRoundedRect(-34, -31, 68, 62, 11)
      graphics.lineStyle(4, 0x6ee7b7, 1)
      graphics.strokeRoundedRect(-34, -31, 68, 62, 11)
      graphics.lineStyle(2, 0x047857, 0.8)
      graphics.strokeCircle(0, 0, 21)
      text.setText('玉').setFontSize(33).setColor('#ecfdf5')
        .setStroke('#047857', 3)
      return
    }

    if (symbol === 'BE') {
      graphics.fillStyle(0xeab308, 1)
      graphics.fillCircle(0, 20, 9)
      graphics.fillRoundedRect(-31, -26, 62, 48, 24)
      graphics.fillStyle(0xfef08a, 1)
      graphics.fillRoundedRect(-21, -19, 42, 10, 5)
      graphics.lineStyle(3, 0xa16207, 1)
      graphics.strokeRoundedRect(-31, -26, 62, 48, 24)
      text.setText('铃').setFontSize(28).setColor('#713f12')
      return
    }

    if (symbol === 'WILD') {
      graphics.fillStyle(0x2563eb, 1)
      graphics.fillRoundedRect(-48, -26, 96, 52, 14)
      graphics.lineStyle(3, 0xf472b6, 1)
      graphics.strokeRoundedRect(-48, -26, 96, 52, 14)
      graphics.fillStyle(0xfef08a, 1)
      graphics.fillTriangle(-43, -25, -28, -36, -18, -25)
      text.setText('WILD').setFontSize(21).setColor('#ffffff')
        .setStroke('#1e3a8a', 4)
      return
    }

    if (symbol === 'SCATTER') {
      graphics.fillStyle(0xdb2777, 1)
      graphics.fillCircle(0, 0, 32)
      graphics.lineStyle(3, 0xfef08a, 1)
      graphics.strokeCircle(0, 0, 32)
      graphics.fillStyle(0xfef08a, 1)
      graphics.fillCircle(-20, -19, 4)
      graphics.fillCircle(20, -19, 4)
      text.setText('福星').setFontSize(20).setColor('#fff7c2')
        .setStroke('#9d174d', 4)
      return
    }

    graphics.fillStyle(0x7c3aed, 1)
    graphics.fillRoundedRect(-47, -27, 94, 54, 11)
    graphics.lineStyle(3, 0xfbbf24, 1)
    graphics.strokeRoundedRect(-47, -27, 94, 54, 11)
    graphics.fillStyle(0xfbbf24, 1)
    graphics.fillRoundedRect(-7, -29, 14, 58, 4)
    graphics.fillCircle(-12, -31, 9)
    graphics.fillCircle(12, -31, 9)
    text.setText('BONUS').setFontSize(18).setColor('#fff7c2')
      .setStroke('#4c1d95', 4)
  }

  private highlightWinningLines(result: SlotSpinServerResult): void {
    this.clearLineHighlights()
    const lines = Array.isArray(result.winning_lines) ? result.winning_lines : []
    for (const line of lines) {
      if (!Array.isArray(line.positions)) continue
      const color = [0xffd84d, 0x22d3ee, 0xfb7185, 0xa3e635][Math.abs(line.line_index) % 4] ?? ACCENT_GOLD
      const path = this.add.graphics().setDepth(24)
      path.lineStyle(5, color, 0.78)
      path.beginPath()
      let hasPoint = false
      for (const position of line.positions) {
        const reel = position?.[0]
        const row = position?.[1]
        if (typeof reel !== 'number' || typeof row !== 'number') continue
        const bg = this.cellBgs[row]?.[reel]
        if (!bg) continue
        bg.setStrokeStyle(3, ACCENT_GOLD, 1)
        bg.setFillStyle(0xffedb0, 1)
        if (hasPoint) path.lineTo(bg.x, bg.y)
        else path.moveTo(bg.x, bg.y)
        hasPoint = true
        path.fillStyle(color, 1)
        path.fillCircle(bg.x, bg.y, 5)
      }
      if (hasPoint) {
        path.strokePath()
        this.lineHighlights.push(path)
      } else {
        path.destroy()
      }
    }
  }

  private clearLineHighlights(): void {
    this.lineHighlights.forEach((marker) => marker.destroy())
    this.lineHighlights = []
  }

  private formatWinningSummary(
    lines: SlotWinningLine[],
    payoutCredits?: string,
    bonusCount = 0,
    fifthReelHasBonus = false,
  ): string | null {
    if (!Array.isArray(lines) || lines.length === 0) return null
    const groups = new Map<string, { symbol: string; count: number }>()
    let total = 0n
    for (const line of lines) {
      if (!line || typeof line.symbol !== 'string' || !Number.isInteger(line.count)) continue
      const key = `${line.symbol}:${line.count}`
      if (!groups.has(key)) groups.set(key, { symbol: line.symbol, count: line.count })
      const payout = this.parseCredits(String(line.payout))
      if (payout !== null) total += payout
    }
    if (groups.size === 0) return null
    const details = Array.from(groups.values(), ({ symbol, count }) => {
      const key = `gameCenter.slot.symbols.${symbol}`
      const translated = this.copy(key)
      const label = translated === key ? slotSymbolLabel(symbol) : translated
      return this.copy('gameCenter.slot.winGroup', { symbol: label, count })
    })
    const finalPayout = this.parseCredits(payoutCredits ?? '') ?? total
    if (fifthReelHasBonus && (bonusCount === 1 || bonusCount === 2)) {
      return this.copy('gameCenter.slot.winBonusSummary', {
        details: details.join(' + '),
        count: bonusCount,
        multiplier: bonusCount === 1 ? '1.5' : '2',
        score: formatCreditsDisplay(finalPayout.toString()),
      })
    }
    return this.copy('gameCenter.slot.winSummary', {
      details: details.join(' + '),
      score: formatCreditsDisplay(finalPayout.toString()),
    })
  }

  private refreshWalletHud(): void {
    const state = this.readSlotState()
    this.creditText.setText(
      this.copy('gameCenter.slot.credits', { credits: formatCreditsDisplay(state.credits) }),
    )
    this.freeSpinText.setText(
      this.copy('gameCenter.slot.freeSpinsLeft', { count: state.freeSpinsRemaining }),
    )
    const walletDefault = this.normalizeBetCredits(state.betCredits)
    if (!this.betInitialized
      || walletDefault !== this.walletDefaultBetCredits
      || state.freeSpinsRemaining > 0) {
      this.selectedBetCredits = walletDefault
    }
    this.walletDefaultBetCredits = walletDefault
    this.betInitialized = true
    this.refreshBetHud(state)

    if (!state.loyaltyEnabled || !state.loyaltyAvailable) {
      this.setMessage(this.copy('gameCenter.slot.maintenance'))
      this.setSpinButtonEnabled(false)
    } else if (!this.spinning && !this.bonusRound) {
      this.setSpinButtonEnabled(true)
    }
    this.refreshAutoSpinButton()
  }

  private refreshBetHud(state: SlotSceneState): void {
    this.betText.setText(
      this.copy('gameCenter.slot.bet', { credits: formatCreditsDisplay(this.selectedBetCredits) }),
    )
    const index = SLOT_BET_OPTIONS.indexOf(this.selectedBetCredits as typeof SLOT_BET_OPTIONS[number])
    const locked = this.spinning
      || this.bonusClaiming
      || Boolean(this.bonusRound)
      || state.freeSpinsRemaining > 0
      || !state.loyaltyEnabled
      || !state.loyaltyAvailable
    this.setBetButtonEnabled(this.betDecreaseButton, !locked && index > 0)
    this.setBetButtonEnabled(this.betIncreaseButton, !locked && index < SLOT_BET_OPTIONS.length - 1)
  }

  private adjustBet(direction: -1 | 1): void {
    const state = this.readSlotState()
    if (this.spinning || this.bonusClaiming || this.bonusRound || state.freeSpinsRemaining > 0) return
    const index = SLOT_BET_OPTIONS.indexOf(this.selectedBetCredits as typeof SLOT_BET_OPTIONS[number])
    const next = SLOT_BET_OPTIONS[index + direction]
    if (!next) return
    this.selectedBetCredits = next
    this.refreshBetHud(state)
  }

  private toggleAutoSpin(): void {
    if (this.autoSpinEnabled) {
      this.stopAutoSpin()
      this.setMessage(this.copy('gameCenter.slot.pressSpin'))
      return
    }
    const state = this.readSlotState()
    if (this.spinning || this.bonusRound || !state.loyaltyEnabled || !state.loyaltyAvailable) return
    this.autoSpinEnabled = true
    this.autoSpinGeneration += 1
    this.refreshAutoSpinButton()
    void this.spin()
  }

  private stopAutoSpin(): void {
    if (!this.autoSpinEnabled) return
    this.autoSpinEnabled = false
    this.autoSpinGeneration += 1
    this.refreshAutoSpinButton()
  }

  private scheduleAutoSpin(): void {
    if (!this.autoSpinEnabled || this.spinning || this.bonusRound) return
    const generation = this.autoSpinGeneration
    this.time.delayedCall(this.reducedMotion ? 80 : AUTO_SPIN_DELAY_MS, () => {
      if (generation !== this.autoSpinGeneration || !this.autoSpinEnabled) return
      void this.spin()
    })
  }

  private refreshAutoSpinButton(): void {
    if (!this.autoSpinButton || !this.autoSpinButtonLabel) return
    const state = this.readSlotState()
    const canToggle = this.autoSpinEnabled
      || (!this.spinning && !this.bonusRound && state.loyaltyEnabled && state.loyaltyAvailable)
    const active = this.autoSpinEnabled
    this.autoSpinButton.setFillStyle(active ? 0x7c2d12 : (canToggle ? 0x6b1526 : 0x4b5563), 1)
    this.autoSpinButton.setStrokeStyle(2, active ? 0xfef3c7 : 0xfbbf24, active ? 1 : 0.75)
    this.autoSpinButtonLabel.setText(this.copy(active
      ? 'gameCenter.slot.autoSpinStop'
      : 'gameCenter.slot.autoSpin'))
    this.autoSpinButtonLabel.setColor(canToggle ? '#fff7c2' : '#d1d5db')
    if (canToggle) this.autoSpinButton.setInteractive({ cursor: 'pointer' })
    else this.autoSpinButton.disableInteractive()
  }

  private setBetButtonEnabled(button: Phaser.GameObjects.Rectangle | undefined, enabled: boolean): void {
    if (!button) return
    button.setFillStyle(enabled ? 0x6b1526 : 0x4b5563, 1)
    if (enabled) button.setInteractive({ cursor: 'pointer' })
    else button.disableInteractive()
  }

  private normalizeBetCredits(value: string): string {
    const normalized = value.trim()
    return (SLOT_BET_OPTIONS as readonly string[]).includes(normalized) ? normalized : '10'
  }

  private canAffordSpin(state: SlotSceneState, betCredits: string): boolean {
    if (state.freeSpinsRemaining > 0) return true
    const credits = this.parseCredits(state.credits)
    const bet = this.parseCredits(betCredits)
    return credits !== null && bet !== null && credits >= bet
  }

  private parseCredits(value: string): bigint | null {
    if (!/^\d+$/.test(value)) return null
    try {
      return BigInt(value)
    } catch {
      return null
    }
  }

  private readSlotState(): SlotSceneState {
    const getter = this.hooks.getSlotState
    if (typeof getter === 'function') {
      const state = getter()
      return {
        credits: state?.credits ?? '0',
        freeSpinsRemaining: state?.freeSpinsRemaining ?? 0,
        betCredits: state?.betCredits ?? '0',
        loyaltyEnabled: Boolean(state?.loyaltyEnabled),
        loyaltyAvailable: Boolean(state?.loyaltyAvailable),
        pendingSlotBonus: state?.pendingSlotBonus,
      }
    }
    return {
      credits: '0',
      freeSpinsRemaining: 0,
      betCredits: '0',
      loyaltyEnabled: false,
      loyaltyAvailable: false,
      pendingSlotBonus: undefined,
    }
  }

  private syncPendingBonus(state: SlotSceneState): void {
    const pending = state.pendingSlotBonus
    if (this.bonusRound) {
      if (pending?.token === this.bonusRound.token) return
      if (this.bonusClaiming) {
        this.queuedBonusOffer = pending ?? null
        return
      }
      this.closeBonusRound(this.copy('gameCenter.slot.pressSpin'))
    }
    if (pending?.token) this.enterBonusRound(pending)
  }

  private hooksRequestSpin(): ((betCredits: string) => Promise<SlotSpinServerResult>) | null {
    const request = this.hooks.requestSlotSpin
    return typeof request === 'function' ? request : null
  }

  private hooksOnResult(result: SlotSpinServerResult): void {
    this.hooks.onSlotResult?.(result)
  }

  private hooksOnError(error: unknown): void {
    this.hooks.onSlotError?.(error)
  }

  private formatSlotError(error: unknown): string {
    try {
      const formatted = this.hooks.formatSlotError?.(error)
      if (formatted) return formatted
    } catch {
      // The built-in message remains available if the host formatter fails.
    }
    if (error && typeof error === 'object') {
      const value = error as { code?: unknown; reason?: unknown }
      if (value.reason === 'GAME_LOYALTY_INSUFFICIENT_CREDITS'
        || value.code === 'GAME_LOYALTY_INSUFFICIENT_CREDITS') {
        return INSUFFICIENT_CREDITS_MESSAGE
      }
    }
    return this.copy('gameCenter.slot.failed')
  }

  private toSceneBonusOffer(
    offer: SlotSpinServerResult['bonus_round'],
  ): SlotSceneBonusOffer | null {
    const token = offer?.token?.trim()
    if (!token || offer?.kind !== 'free_spins') return null
    const bonusCount = Number.isInteger(offer.bonus_count) ? offer.bonus_count : 3
    const derivedMultiplier = bonusCount === 5 ? 4 : bonusCount === 4 ? 2 : 1
    return {
      token,
      kind: 'free_spins',
      expiresAt: offer.expires_at,
      betCredits: this.normalizeBetCredits(offer.bet_credits),
      bonusCount,
      bonusMultiplier: Number.isInteger(offer.bonus_multiplier)
        ? offer.bonus_multiplier
        : derivedMultiplier,
      options: this.normalizeBonusOptions(offer.options),
    }
  }

  private normalizeBonusOptions(options: SlotBonusOption[] | undefined): SlotBonusOption[] {
    return DEFAULT_BONUS_OPTIONS.map((fallback) => {
      const option = options?.find((candidate) => candidate?.choice === fallback.choice)
      if (!option || option.spins !== fallback.spins || option.multiplier !== fallback.multiplier) {
        return { ...fallback }
      }
      return { choice: option.choice, spins: option.spins, multiplier: option.multiplier }
    })
  }

  private enterBonusRound(offer: SlotSceneBonusOffer): void {
    if (this.bonusRound || !this.hooks.requestSlotBonus) return
    const generation = ++this.bonusGeneration
    this.bonusClaiming = false
    this.setSpinButtonEnabled(false)

    const container = this.add.container(0, 0).setDepth(100)
    const shade = this.add.rectangle(GAME_WIDTH / 2, SCENE_HEIGHT / 2, GAME_WIDTH, SCENE_HEIGHT, 0x17062f, 0.94)
    const panel = this.add.rectangle(450, 300, 760, 474, 0x3b0b63, 1)
      .setStrokeStyle(4, ACCENT_GOLD, 0.95)
    const inner = this.add.rectangle(450, 300, 738, 452, 0x6b1526, 0.72)
      .setStrokeStyle(2, 0xffd76a, 0.62)
    const title = this.add.text(450, 106, this.copy('gameCenter.slot.bonusTitle'), {
      color: '#ffe9a6',
      fontFamily: '"Noto Sans SC", ui-sans-serif, sans-serif',
      fontSize: '36px',
      fontStyle: 'bold',
      stroke: '#7f1d1d',
      strokeThickness: 6,
    }).setOrigin(0.5)
    const multiplierNote = this.add.text(450, 158, this.copy('gameCenter.slot.bonusMultiplier', {
      count: offer.bonusCount,
      multiplier: offer.bonusMultiplier,
    }), {
      color: '#fcd34d',
      fontFamily: '"Noto Sans SC", ui-sans-serif, sans-serif',
      fontSize: '19px',
      fontStyle: 'bold',
      align: 'center',
    }).setOrigin(0.5)
    const message = this.add.text(450, 206, this.copy('gameCenter.slot.bonusPrompt'), {
      color: '#fff7d6',
      fontFamily: '"Noto Sans SC", ui-sans-serif, sans-serif',
      fontSize: '18px',
      align: 'center',
      wordWrap: { width: 580 },
    }).setOrigin(0.5)

    const choices = offer.options.map((option, index) => (
      this.createBonusChoice(250 + index * 200, 346, option)
    ))
    const returnBg = this.add.rectangle(0, 0, 132, 38, 0x120d0d, 1)
      .setStrokeStyle(1, ACCENT_GOLD, 0.75)
    const returnLabel = this.add.text(0, 0, this.copy('gameCenter.slot.bonusBack'), {
      color: '#fde68a',
      fontFamily: '"Noto Sans SC", ui-sans-serif, sans-serif',
      fontSize: '15px',
      fontStyle: 'bold',
    }).setOrigin(0.5)
    const returnButton = this.add.container(450, 522, [returnBg, returnLabel])
      .setSize(132, 38)
      .setVisible(false)
      .setInteractive({ cursor: 'pointer' })
    returnButton.on('pointerdown', () => this.closeBonusRound(this.copy('gameCenter.slot.pressSpin')))

    container.add([
      shade,
      panel,
      inner,
      title,
      multiplierNote,
      message,
      ...choices.map((choice) => choice.container),
      returnButton,
    ])
    this.bonusRound = {
      token: offer.token,
      offer,
      generation,
      container,
      message,
      choices,
      returnButton,
    }
    this.refreshBetHud(this.readSlotState())
    this.emitSound('slots-bonus-enter')

    if (!this.reducedMotion && choices.length > 0) {
      this.tweens.add({
        targets: choices.map((choice) => choice.container),
        y: '-=8',
        duration: 520,
        yoyo: true,
        repeat: -1,
        ease: 'Sine.easeInOut',
        stagger: 110,
      })
    }
  }

  private createBonusChoice(x: number, y: number, option: SlotBonusOption): BonusChoiceView {
    const background = this.add.rectangle(0, 0, 176, 104, 0x7f1d1d, 1)
      .setStrokeStyle(3, 0xffd85e, 1)
    const label = this.add.text(0, 0, this.copy('gameCenter.slot.bonusOption', {
      spins: option.spins,
      multiplier: option.multiplier,
    }), {
      color: '#fff7c2',
      fontFamily: '"Noto Sans SC", ui-sans-serif, sans-serif',
      fontSize: '23px',
      fontStyle: 'bold',
    }).setOrigin(0.5)
    const container = this.add.container(x, y, [background, label])
      .setSize(176, 104)
      .setInteractive({ cursor: 'pointer' })
    container.on('pointerover', () => {
      if (!this.bonusClaiming && !this.reducedMotion) container.setScale(1.06)
    })
    container.on('pointerout', () => {
      if (!this.bonusClaiming) container.setScale(1)
    })
    container.on('pointerdown', () => void this.claimBonus(option.choice))
    return { container, background, label, option }
  }

  private async claimBonus(choice: number): Promise<void> {
    const round = this.bonusRound
    const request = this.hooks.requestSlotBonus
    if (!round || !request || this.bonusClaiming) return

    this.bonusClaiming = true
    round.message.setText(this.copy('gameCenter.slot.bonusWaiting'))
    round.returnButton.setVisible(false)
    round.choices.forEach((view) => {
      view.container.disableInteractive().setScale(1)
      view.background.setFillStyle(view.option.choice === choice ? 0xb45309 : 0x4b5563, 1)
    })
    this.emitSound('slots-bonus-pick')

    let result: SlotBonusServerResult
    try {
      result = await request(round.token, choice)
    } catch (error: unknown) {
      if (!this.bonusRound || this.bonusRound.generation !== round.generation) return
      this.bonusClaiming = false
      round.message.setText(`${this.formatSlotError(error)} · 领取失败，请重试`)
      round.returnButton.setVisible(false)
      round.choices.forEach((view) => {
        view.background.setFillStyle(0x7f1d1d, 1)
        view.container.setInteractive({ cursor: 'pointer' })
      })
      this.hooks.onSlotBonusError?.(error)
      return
    }

    if (!this.bonusRound || this.bonusRound.generation !== round.generation) return
    try {
      await this.playBonusSpins(result, round)
    } catch {
      // Settlement already succeeded; a presentation failure must not retry the claim.
    }
    if (!this.bonusRound || this.bonusRound.generation !== round.generation) return
    this.finishBonusResult(result)
  }

  private async playBonusSpins(
    result: SlotBonusServerResult,
    round: BonusRoundView,
  ): Promise<void> {
    const spins = Array.isArray(result.spin_results) ? result.spin_results : []
    round.container.setVisible(false)
    for (let index = 0; index < spins.length; index += 1) {
      if (this.bonusRound?.generation !== round.generation) return
      const spin = this.normalizeBonusSpin(spins[index]!, result, round, index)
      await this.playBonusSpin(spin, index + 1, spins.length, round)
    }
  }

  private normalizeBonusSpin(
    raw: SlotBonusSpinResult,
    claim: SlotBonusServerResult,
    round: BonusRoundView,
    index: number,
  ): SlotSpinServerResult {
    const winningLines: SlotWinningLine[] = Array.isArray(raw.winning_lines)
      ? raw.winning_lines.map((line, lineIndex) => ({
          line_index: Number.isInteger(line.line_index) ? line.line_index : lineIndex + 1,
          symbol: typeof line.symbol === 'string' ? line.symbol : 'BE',
          count: Number.isInteger(line.count) ? line.count : 0,
          multiplier: String(line.multiplier ?? '0'),
          payout: String(line.payout ?? '0'),
          positions: Array.isArray(line.positions)
            ? line.positions.filter((position): position is [number, number] => (
                Array.isArray(position)
                && typeof position[0] === 'number'
                && typeof position[1] === 'number'
              ))
            : [],
        }))
      : []
    const payout = String(raw.payout_credits ?? raw.total_payout ?? '0')
    return {
      id: raw.id ?? `${claim.round_id}-spin-${index + 1}`,
      bet_credits: String(raw.bet_credits ?? round.offer.betCredits),
      payout_credits: payout,
      used_free_spin: true,
      grid: normalizeServerGrid(raw.grid),
      stops: Array.isArray(raw.stops) ? raw.stops : [],
      winning_lines: winningLines,
      scatter_count: Number(raw.scatter_count ?? 0),
      bonus_count: Number(raw.bonus_count ?? 0),
      free_spins_awarded: Number(raw.free_spins_awarded ?? 0),
      free_spins_remaining: Math.max(0, claim.spins - index - 1),
      credits_before: raw.credits_before ?? claim.credits_after,
      credits_after: raw.credits_after ?? claim.credits_after,
      paytable_version: raw.paytable_version ?? 'slot-paytable-v7',
      reel_strip_version: raw.reel_strip_version ?? 'slot-reels-v5',
      idempotency_key: raw.idempotency_key ?? `${claim.round_id}-${index + 1}`,
      idempotent_replay: raw.idempotent_replay,
      created_at: raw.created_at ?? claim.claimed_at,
    }
  }

  private async playBonusSpin(
    spin: SlotSpinServerResult,
    current: number,
    total: number,
    round: BonusRoundView,
  ): Promise<void> {
    const progress = this.copy('gameCenter.slot.bonusSpinProgress', { current, total })
    round.message.setText(progress)
    this.setMessage(progress)
    this.clearLineHighlights()
    const grid = normalizeServerGrid(spin.grid)
    if (!this.reducedMotion) {
      for (let reel = 0; reel < SLOT_REEL_COUNT; reel += 1) {
        const symbols = randomRollingReelSymbols(reel)
        for (let row = 0; row < SLOT_ROW_COUNT; row += 1) {
          this.paintCell(row, reel, symbols[row]!, false)
        }
        await this.waitForSceneTime(BONUS_REEL_STOP_DELAY_MS)
        for (let row = 0; row < SLOT_ROW_COUNT; row += 1) {
          this.paintCell(row, reel, grid[row]![reel]!, true)
        }
        this.emitSound('slots-reel-stop')
      }
    } else {
      this.applyGrid(grid, true)
    }
    this.lastAppliedGrid = grid
    this.highlightWinningLines(spin)
    const detail = this.formatWinningSummary(
      spin.winning_lines,
      spin.payout_credits,
      spin.bonus_count ?? 0,
      hasBonusOnFifthReel(grid),
    )
      ?? (hasPositivePayout(spin.payout_credits)
        ? this.copy('gameCenter.slot.win', { payout: formatCreditsDisplay(spin.payout_credits) })
        : this.copy('gameCenter.slot.noWin'))
    round.message.setText(`${progress} · ${detail}`)
    this.setMessage(`${progress} · ${detail}`)
    this.emitSound(hasPositivePayout(spin.payout_credits) ? 'slots-win' : 'slots-no-win')
    await this.waitForSceneTime(this.reducedMotion ? 90 : BONUS_RESULT_DISPLAY_MS)
  }

  private waitForSceneTime(delay: number): Promise<void> {
    return new Promise((resolve) => {
      this.time.delayedCall(delay, resolve)
    })
  }

  private finishBonusResult(result: SlotBonusServerResult): void {
    const round = this.bonusRound
    if (!round) return
    const rewardMessage = this.copy('gameCenter.slot.bonusReward', {
      credits: formatCreditsDisplay(result.reward_credits),
      multiplier: result.bonus_multiplier,
    })
    round.container.setVisible(true)
    round.message.setText(rewardMessage)
    round.choices.forEach((view) => {
      view.container.setAlpha(view.option.choice === result.choice ? 1 : 0.35)
    })
    this.creditText.setText(
      this.copy('gameCenter.slot.credits', { credits: formatCreditsDisplay(result.credits_after) }),
    )
    this.hooks.onSlotBonusResult?.(result)
    this.emitSound('slots-bonus-reveal')
    this.time.delayedCall(this.reducedMotion ? 900 : 2200, () => {
      if (this.bonusRound?.generation === round.generation) this.closeBonusRound(rewardMessage)
    })
  }

  private closeBonusRound(message: string): void {
    if (!this.bonusRound) return
    const nextBonus = this.queuedBonusOffer ?? this.readSlotState().pendingSlotBonus ?? null
    this.queuedBonusOffer = null
    this.bonusRound.container.destroy(true)
    this.bonusRound = null
    this.bonusClaiming = false
    this.bonusGeneration += 1
    this.refreshWalletHud()
    this.setMessage(message)
    if (nextBonus?.token) this.enterBonusRound(nextBonus)
  }

  private resolveWinCelebrationTier(payoutCredits: string, betCredits: string): WinCelebrationTier {
    if (!isBigWinPayout(payoutCredits, betCredits)) return 'regular'
    const payout = this.parseCredits(payoutCredits)
    const bet = this.parseCredits(betCredits)
    if (payout !== null && bet !== null && bet > 0n && payout >= bet * SLOT_JACKPOT_WIN_MULTIPLIER) return 'jackpot'
    return 'big'
  }

  private celebrateWin(tier: WinCelebrationTier): void {
    this.clearCelebration()
    const generation = this.celebrationGeneration
    this.marqueeBulbs.forEach((bulb) => bulb.setFillStyle(0xfff3a3, 1))
    const label = tier === 'jackpot'
      ? this.copy('gameCenter.slot.jackpot')
      : tier === 'big'
        ? this.copy('gameCenter.slot.bigWin')
        : this.copy('gameCenter.slot.win', { payout: '' }).replace('+', '').trim()
    const banner = this.add.text(450, tier === 'regular' ? 280 : 300, label, {
      color: '#fff7c2',
      fontFamily: '"Noto Sans SC", ui-sans-serif, sans-serif',
      fontSize: tier === 'regular' ? '34px' : tier === 'big' ? '48px' : '62px',
      fontStyle: 'bold',
      stroke: '#991b1b',
      strokeThickness: tier === 'regular' ? 6 : 9,
    }).setOrigin(0.5).setDepth(40)
    this.celebrationObjects.add(banner)
    if (this.reducedMotion) {
      this.time.delayedCall(650, () => this.destroyCelebrationObject(banner))
      return
    }

    this.spawnCoinRain(tier)
    banner.setScale(tier === 'regular' ? 0.82 : 0.62)
    this.tweens.add({
      targets: banner,
      scale: tier === 'regular' ? 1 : 1.08,
      alpha: { from: 0.35, to: 1 },
      duration: tier === 'regular' ? 180 : 260,
      yoyo: true,
      hold: tier === 'regular' ? 240 : tier === 'big' ? 620 : 900,
      onComplete: () => this.destroyCelebrationObject(banner),
    })
    this.time.delayedCall(tier === 'jackpot' ? 1450 : 900, () => {
      if (generation !== this.celebrationGeneration) return
      this.marqueeBulbs.forEach((bulb, index) => bulb.setFillStyle(index % 2 ? ACCENT_RED : ACCENT_GOLD, 0.95))
    })
  }

  private spawnCoinRain(tier: WinCelebrationTier): void {
    if (this.reducedMotion) return
    this.ensureCoinTexture()
    const count = tier === 'regular' ? 14 : tier === 'big' ? 46 : 78
    const spreadDuration = tier === 'regular' ? 650 : tier === 'big' ? 1250 : 1900
    for (let index = 0; index < count; index += 1) {
      const startX = Phaser.Math.Between(72, GAME_WIDTH - 72)
      const coin = this.add.image(startX, Phaser.Math.Between(-80, -18), COIN_TEXTURE_KEY)
        .setDepth(52)
        .setScale(Phaser.Math.FloatBetween(0.7, tier === 'jackpot' ? 1.25 : 1.05))
        .setAngle(Phaser.Math.Between(-35, 35))
      this.celebrationObjects.add(coin)
      this.tweens.add({
        targets: coin,
        x: startX + Phaser.Math.Between(-105, 105),
        y: Phaser.Math.Between(470, 625),
        angle: coin.angle + Phaser.Math.Between(240, 760),
        alpha: { from: 1, to: 0.18 },
        delay: Phaser.Math.Between(0, spreadDuration),
        duration: Phaser.Math.Between(820, 1350),
        ease: 'Cubic.easeIn',
        onComplete: () => this.destroyCelebrationObject(coin),
      })
    }
  }

  private ensureCoinTexture(): void {
    if (this.textures.exists(COIN_TEXTURE_KEY)) return
    const coin = this.add.graphics().setVisible(false)
    coin.fillStyle(0xf59e0b, 1)
    coin.fillCircle(11, 11, 10)
    coin.lineStyle(2, 0xfff1a8, 1)
    coin.strokeCircle(11, 11, 9)
    coin.fillStyle(0x9a3412, 0.82)
    coin.fillRect(8, 8, 6, 6)
    coin.generateTexture(COIN_TEXTURE_KEY, 22, 22)
    coin.destroy()
  }

  private destroyCelebrationObject(object: Phaser.GameObjects.GameObject): void {
    this.celebrationObjects.delete(object)
    if (object.active) object.destroy()
  }

  private clearCelebration(): void {
    this.celebrationGeneration += 1
    this.celebrationObjects.forEach((object) => {
      this.tweens?.killTweensOf(object)
      if (object.active) object.destroy()
    })
    this.celebrationObjects.clear()
    this.marqueeBulbs.forEach((bulb, index) => bulb.setFillStyle(index % 2 ? ACCENT_RED : ACCENT_GOLD, 0.95))
  }

  private resolveReducedMotion(): boolean {
    const hook = this.hooks.prefersReducedMotion
    if (typeof hook === 'function') return Boolean(hook())
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
    try {
      return window.matchMedia('(prefers-reduced-motion: reduce)').matches
    } catch {
      return false
    }
  }

  private copy(key: string, params: Record<string, string | number> = {}): string {
    const hook = this.hooks.t
    let template = DEFAULT_COPY[key] ?? key
    if (typeof hook === 'function') {
      try {
        const resolved = hook(key, params)
        if (typeof resolved === 'string' && resolved.length > 0 && resolved !== key) {
          template = resolved
        }
      } catch {
        // Fall back to built-in Chinese copy.
      }
    }
    return Object.entries(params).reduce(
      (text, [name, value]) => text.replace(new RegExp(`\\{${name}\\}`, 'g'), String(value)),
      template,
    )
  }

  private setMessage(text: string): void {
    this.messageText.setText(text)
  }

  private setSpinButtonEnabled(enabled: boolean): void {
    this.spinButton.setFillStyle(enabled ? ACCENT_RED : 0x6b7280, 1)
    this.spinButtonLabel.setColor(enabled ? '#fff7c2' : '#d1d5db')
    if (enabled) {
      this.spinButton.setInteractive({ cursor: 'pointer' })
    } else {
      this.spinButton.disableInteractive()
    }
  }

  private drawMacauCabinet(): void {
    this.cameras.main.setBackgroundColor('#2a0b52')
    const backdrop = this.add.graphics()
    backdrop.fillStyle(0x2a0b52, 1)
    backdrop.fillRect(0, 0, GAME_WIDTH, SCENE_HEIGHT)
    backdrop.fillStyle(0x4c1d95, 1)
    backdrop.fillRect(0, 0, GAME_WIDTH, 126)
    backdrop.fillStyle(0x1e093d, 1)
    backdrop.fillRect(0, 430, GAME_WIDTH, 170)
    backdrop.fillStyle(0xc026d3, 0.12)
    backdrop.fillTriangle(70, 0, 325, 430, 205, 430)
    backdrop.fillStyle(0xf59e0b, 0.1)
    backdrop.fillTriangle(820, 0, 590, 430, 720, 430)
    for (const [x, y, radius] of [
      [28, 38, 2], [94, 20, 3], [151, 72, 2], [745, 34, 2], [802, 76, 3], [875, 24, 2],
      [18, 182, 2], [873, 198, 2], [42, 388, 3], [856, 402, 2],
    ] as Array<[number, number, number]>) {
      backdrop.fillStyle(0xfff7c2, 0.9)
      backdrop.fillCircle(x, y, radius)
    }

    const frame = this.add.graphics()
    frame.fillStyle(0xf59e0b, 1)
    frame.fillRoundedRect(34, 30, 832, 548, 22)
    frame.lineStyle(5, 0xffef9f, 0.95)
    frame.strokeRoundedRect(34, 30, 832, 548, 22)
    frame.lineStyle(3, 0xb45309, 1)
    frame.strokeRoundedRect(44, 40, 812, 528, 17)

    const cabinet = this.add.graphics()
    cabinet.fillStyle(0x7f1d1d, 1)
    cabinet.fillRoundedRect(49, 45, 802, 518, 16)
    cabinet.lineStyle(3, 0xffdc73, 0.9)
    cabinet.strokeRoundedRect(55, 51, 790, 506, 13)
    cabinet.fillStyle(CABINET_RED, 1)
    cabinet.fillRoundedRect(64, 65, 772, 478, 11)
    cabinet.fillStyle(0xef4444, 0.7)
    cabinet.fillRoundedRect(76, 77, 748, 50, 9)
    cabinet.fillStyle(PANEL_BLACK, 1)
    cabinet.fillRoundedRect(90, 146, 720, 274, 8)
    cabinet.lineStyle(5, 0xf59e0b, 1)
    cabinet.strokeRoundedRect(90, 146, 720, 274, 8)
    cabinet.lineStyle(2, 0xffef9f, 0.85)
    cabinet.strokeRoundedRect(97, 153, 706, 260, 5)
    cabinet.fillStyle(0x4c0f22, 1)
    cabinet.fillRoundedRect(84, 428, 732, 116, 7)
    cabinet.lineStyle(2, 0xfbbf24, 0.7)
    cabinet.strokeRoundedRect(84, 428, 732, 116, 7)

    const marquee = this.add.graphics()
    marquee.fillStyle(0x6d1325, 1)
    marquee.fillRoundedRect(198, 69, 504, 67, 12)
    marquee.lineStyle(5, ACCENT_GOLD, 1)
    marquee.strokeRoundedRect(198, 69, 504, 67, 12)
    marquee.lineStyle(2, 0xffefaa, 0.9)
    marquee.strokeRoundedRect(207, 78, 486, 49, 8)
    this.add.text(450, 102, this.copy('gameCenter.slot.title'), {
      color: '#fff1a8',
      fontFamily: '"Noto Sans SC", ui-sans-serif, sans-serif',
      fontSize: '31px',
      fontStyle: 'bold',
      stroke: '#991b1b',
      strokeThickness: 7,
    }).setOrigin(0.5)

    const bulbPositions: Array<[number, number]> = []
    for (let x = 62; x <= 838; x += 28) {
      bulbPositions.push([x, 50], [x, 558])
    }
    for (let y = 78; y <= 530; y += 28) {
      bulbPositions.push([49, y], [851, y])
    }
    this.marqueeBulbs = bulbPositions.map(([x, y], index) => this.add.circle(
      x,
      y,
      4.5,
      index % 2 ? ACCENT_RED : ACCENT_GOLD,
      0.95,
    ).setStrokeStyle(1, 0xffe9a3, 0.65))
    if (!this.reducedMotion) {
      this.tweens.add({
        targets: this.marqueeBulbs.filter((_, index) => index % 2 === 0),
        alpha: 0.35,
        duration: 360,
        yoyo: true,
        repeat: -1,
      })
      this.tweens.add({
        targets: this.marqueeBulbs.filter((_, index) => index % 2 === 1),
        alpha: 0.35,
        duration: 360,
        delay: 180,
        yoyo: true,
        repeat: -1,
      })
    }

    this.add.text(84, 105, this.copy('gameCenter.slot.ways'), {
      color: '#fff1a8',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '13px',
      fontStyle: 'bold',
    }).setOrigin(0.5)

    for (let index = 0; index < 10; index += 1) {
      const y = 168 + index * 25.5
      const left = this.add.circle(73, y, 9, index % 2 ? 0x2563eb : 0xdb2777, 1)
        .setStrokeStyle(2, 0xffef9f, 0.95)
      const right = this.add.circle(827, y, 9, index % 2 ? 0x10b981 : 0xf59e0b, 1)
        .setStrokeStyle(2, 0xffef9f, 0.95)
      this.add.text(left.x, left.y, String(index + 1), {
        color: '#ffffff', fontFamily: 'ui-monospace, monospace', fontSize: '8px', fontStyle: 'bold',
      }).setOrigin(0.5)
      this.add.text(right.x, right.y, String(index + 11), {
        color: '#ffffff', fontFamily: 'ui-monospace, monospace', fontSize: '8px', fontStyle: 'bold',
      }).setOrigin(0.5)
    }
  }

  private buildReelWindow(): void {
    const maskShape = this.make.graphics({ x: 0, y: 0 })
    maskShape.fillStyle(0xffffff, 1)
    maskShape.fillRect(104, 160, 692, 252)
    const reelMask = maskShape.createGeometryMask()

    for (let row = 0; row < SLOT_ROW_COUNT; row += 1) {
      const rowTexts: Phaser.GameObjects.Text[] = []
      const rowBgs: Phaser.GameObjects.Rectangle[] = []
      const rowIcons: Phaser.GameObjects.Graphics[] = []
      for (let reel = 0; reel < SLOT_REEL_COUNT; reel += 1) {
        const x = REEL_ORIGIN_X + reel * (REEL_CELL_WIDTH + REEL_GAP_X) + REEL_CELL_WIDTH / 2
        const y = REEL_ORIGIN_Y + row * (REEL_CELL_HEIGHT + REEL_GAP_Y) + REEL_CELL_HEIGHT / 2
        const bg = this.add.rectangle(x, y, REEL_CELL_WIDTH, REEL_CELL_HEIGHT, 0xfff8df, 1)
          .setStrokeStyle(1, 0xd5a63a, 0.75)
          .setMask(reelMask)
        const icon = this.add.graphics().setPosition(x, y).setMask(reelMask)
        const text = this.add.text(x, y, slotSymbolLabel('BE'), {
          color: '#713f12',
          fontFamily: '"Noto Sans SC", ui-sans-serif, sans-serif',
          fontSize: '24px',
          fontStyle: 'bold',
          align: 'center',
          wordWrap: { width: REEL_CELL_WIDTH - 12 },
        }).setOrigin(0.5).setMask(reelMask)
        rowBgs.push(bg)
        rowIcons.push(icon)
        rowTexts.push(text)
      }
      this.cellBgs.push(rowBgs)
      this.cellIcons.push(rowIcons)
      this.cellTexts.push(rowTexts)
    }

    const glass = this.add.graphics()
    glass.fillStyle(0xffffff, 0.055)
    glass.fillRect(104, 160, 692, 38)
    glass.fillStyle(0x000000, 0.26)
    glass.fillRect(104, 386, 692, 26)
    glass.lineStyle(2, 0xffd96a, 0.55)
    for (let reel = 1; reel < SLOT_REEL_COUNT; reel += 1) {
      const x = REEL_ORIGIN_X + reel * (REEL_CELL_WIDTH + REEL_GAP_X) - REEL_GAP_X / 2
      glass.lineBetween(x, 160, x, 412)
    }
  }

  private buildHud(): void {
    const displays = this.add.graphics()
    for (const x of [112, 306, 500]) {
      displays.fillStyle(0x1c1024, 1)
      displays.fillRoundedRect(x, 445, 176, 42, 5)
      displays.lineStyle(1, 0xfbbf24, 0.7)
      displays.strokeRoundedRect(x, 445, 176, 42, 5)
    }
    const font = {
      color: '#fde68a',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, "Noto Sans SC", sans-serif',
      fontSize: '14px',
      fontStyle: 'bold' as const,
    }

    this.creditText = this.add.text(124, 466, this.copy('gameCenter.slot.credits', { credits: '0' }), font)
      .setOrigin(0, 0.5)
    this.freeSpinText = this.add.text(318, 466, this.copy('gameCenter.slot.freeSpinsLeft', { count: 0 }), {
      ...font,
      color: '#f9a8d4',
    }).setOrigin(0, 0.5)
    this.betDecreaseButton = this.add.rectangle(520, 466, 32, 30, 0x6b1526, 1)
      .setStrokeStyle(1, 0xfbbf24, 0.8)
      .setInteractive({ cursor: 'pointer' })
    this.betIncreaseButton = this.add.rectangle(620, 466, 32, 30, 0x6b1526, 1)
      .setStrokeStyle(1, 0xfbbf24, 0.8)
      .setInteractive({ cursor: 'pointer' })
    this.add.text(520, 464, '-', {
      ...font,
      color: '#fff7c2',
      fontSize: '22px',
    }).setOrigin(0.5)
    this.add.text(620, 465, '+', {
      ...font,
      color: '#fff7c2',
      fontSize: '20px',
    }).setOrigin(0.5)
    this.betText = this.add.text(570, 466, this.copy('gameCenter.slot.bet', { credits: '0' }), {
      ...font,
      color: '#86efac',
      fontSize: '12px',
    }).setOrigin(0.5)
    this.betDecreaseButton.on('pointerdown', () => this.adjustBet(-1))
    this.betIncreaseButton.on('pointerdown', () => this.adjustBet(1))

    this.messageText = this.add.text(365, 520, '', {
      color: '#fff7d6',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, "Noto Sans SC", sans-serif',
      fontSize: '14px',
      align: 'center',
      wordWrap: { width: 530 },
      maxLines: 2,
    }).setOrigin(0.5)

    this.spinButton = this.add.rectangle(724, 474, 160, 58, ACCENT_RED, 1)
      .setInteractive({ cursor: 'pointer' })
    this.spinButton.setStrokeStyle(4, 0xffefaa, 1)
    this.spinButtonLabel = this.add.text(724, 474, this.copy('gameCenter.slot.spin'), {
      color: '#fff7c2',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, "Noto Sans SC", sans-serif',
      fontSize: '22px',
      fontStyle: 'bold',
    }).setOrigin(0.5)
    this.spinButton.on('pointerdown', () => {
      void this.spin()
    })

    this.autoSpinButton = this.add.rectangle(724, 520, 124, 28, 0x6b1526, 1)
      .setStrokeStyle(2, 0xfbbf24, 0.75)
      .setInteractive({ cursor: 'pointer' })
    this.autoSpinButtonLabel = this.add.text(724, 520, this.copy('gameCenter.slot.autoSpin'), {
      color: '#fff7c2',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, "Noto Sans SC", sans-serif',
      fontSize: '13px',
      fontStyle: 'bold',
    }).setOrigin(0.5)
    this.autoSpinButton.on('pointerdown', () => this.toggleAutoSpin())
  }
}

// Re-export for tests that need the idle layout.
export const LUCKY_IDLE_GRID = IDLE_GRID
