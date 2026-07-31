import Phaser from 'phaser'
import { ArcadeScene } from '../ArcadeScene'
import { GAME_HEIGHT, GAME_WIDTH } from '../ArcadeScene'
import type {
  FruitBets,
  FruitDoor,
  FruitRewardStop,
  FruitSpinServerResult,
  GameControl,
  SceneHooks,
} from '../../types'

const DOORS: FruitDoor[] = ['bar', '77', 'star', 'watermelon', 'bell', 'mango', 'orange', 'apple']

const LABELS: Record<FruitDoor, string> = {
  bar: 'BAR',
  '77': '9',
  star: '星星',
  watermelon: '西瓜',
  bell: '铃铛',
  mango: '李子',
  orange: '橙子',
  apple: '苹果',
}

const ASSET_FILES: Record<FruitDoor, string> = {
  bar: 'bar.webp',
  '77': 'nine.webp',
  star: 'star.webp',
  watermelon: 'watermelon.webp',
  bell: 'bell.webp',
  mango: 'plum.webp',
  orange: 'orange.webp',
  apple: 'apple.webp',
}

const BET_COPY: Record<FruitDoor, string> = {
  bar: '60 / 120 / 80',
  '77': '大40 小3',
  star: '大40 小3',
  watermelon: '大40 小3',
  bell: '大20 小3',
  mango: '大20 小3',
  orange: '大20 小3',
  apple: '大6',
}

export interface FruitReferenceCell {
  symbol?: FruitDoor
  size?: 'big' | 'small'
  multiplier?: number
  printed: string
  special?: true
}

export const FRUIT_REFERENCE_CELLS: readonly FruitReferenceCell[] = [
  { symbol: 'orange', size: 'big', multiplier: 20, printed: '10-20' },
  { symbol: 'bell', size: 'big', multiplier: 20, printed: '10-20' },
  { symbol: 'bar', size: 'big', multiplier: 60, printed: '×60' },
  { symbol: 'bar', size: 'big', multiplier: 120, printed: '×120' },
  { symbol: 'bar', size: 'big', multiplier: 80, printed: '×80' },
  { symbol: 'apple', size: 'big', multiplier: 6, printed: '5-6' },
  { symbol: 'mango', size: 'big', multiplier: 20, printed: '10-20' },
  { symbol: 'watermelon', size: 'big', multiplier: 40, printed: '20-40' },
  { symbol: 'watermelon', size: 'small', multiplier: 3, printed: '×3' },
  { printed: '金猪', special: true },
  { symbol: 'apple', size: 'big', multiplier: 6, printed: '5-6' },
  { symbol: 'orange', size: 'small', multiplier: 3, printed: '×3' },
  { symbol: 'orange', size: 'big', multiplier: 20, printed: '10-20' },
  { symbol: 'bell', size: 'big', multiplier: 20, printed: '10-20' },
  { symbol: '77', size: 'small', multiplier: 3, printed: '×3' },
  { symbol: '77', size: 'big', multiplier: 40, printed: '20-40' },
  { symbol: 'apple', size: 'big', multiplier: 6, printed: '5-6' },
  { symbol: 'mango', size: 'small', multiplier: 3, printed: '×3' },
  { symbol: 'mango', size: 'big', multiplier: 20, printed: '10-20' },
  { symbol: 'star', size: 'big', multiplier: 40, printed: '20-40' },
  { symbol: 'star', size: 'small', multiplier: 3, printed: '×3' },
  { printed: '金猪', special: true },
  { symbol: 'apple', size: 'big', multiplier: 6, printed: '5-6' },
  { symbol: 'bell', size: 'small', multiplier: 3, printed: '×3' },
]

interface CenterCell {
  label: string
  symbol?: FruitDoor
  accent?: number
}

const CENTER_WHEEL: readonly CenterCell[] = [
  { label: '金猪\n再转', accent: 0xf4be45 },
  { label: '9', symbol: '77' },
  { label: '双响炮', accent: 0xe85d3f },
  { label: '星星', symbol: 'star' },
  { label: '彩金', accent: 0xf7c64a },
  { label: '西瓜', symbol: 'watermelon' },
  { label: '大四喜', accent: 0xe85d3f },
  { label: '铃铛', symbol: 'bell' },
  { label: '谢谢\n有奖', accent: 0x68a37e },
  { label: '李子', symbol: 'mango' },
  { label: '小三元', accent: 0xf4be45 },
  { label: '橙子', symbol: 'orange' },
  { label: '大三元', accent: 0xe85d3f },
  { label: '苹果', symbol: 'apple' },
  { label: '开火车', accent: 0x68a37e },
  { label: 'BAR', symbol: 'bar' },
]

const CENTER_DISPLAY_VALUES = ['2222', '3333', '5555', '6666', '7777', '8888', '9999', '2376', '3168'] as const
const CENTER_PULSE_COLORS = [0x43d17d, 0x4389e8, 0xe94f55, 0xf0c94f] as const

export const FRUIT_ANIMATION_TIMING = {
  centerPreludeSteps: 65,
  centerPreludeStepMs: 112,
  outerLaps: 3,
  outerRunMs: 5560,
  centerLaps: 3,
  centerRunMs: 6960,
  landingFlashMs: 85,
} as const

export const FRUIT_BET_HOLD_TIMING = {
  initialDelayMs: 300,
  repeatMs: 60,
} as const

export const FRUIT_TRAIN_TIMING = {
  firstRunLaps: 2,
  nextRunLaps: 1,
  firstRunMs: 3200,
  nextRunMs: 1850,
  rewardHoldMs: 320,
  trailLength: 5,
} as const

const MAX_BET_PER_DOOR = 100

export function createFruitRunDelays(steps: number, totalMs: number): number[] {
  const safeSteps = Math.max(1, Math.floor(steps))
  const safeTotal = Math.max(safeSteps, Math.floor(totalMs))
  const weights = Array.from({ length: safeSteps }, (_, index) => {
    const progress = index / Math.max(1, safeSteps - 1)
    return 0.55 + Math.pow(progress, 2.7) * 2.45
  })
  const weightTotal = weights.reduce((sum, weight) => sum + weight, 0)
  const delays = weights.map((weight) => Math.max(1, Math.round(safeTotal * weight / weightTotal)))
  const difference = safeTotal - delays.reduce((sum, delay) => sum + delay, 0)
  delays[delays.length - 1] = Math.max(1, delays[delays.length - 1] + difference)
  return delays
}

export function fruitTimelineWaitMs(startedAtMs: number, scheduledElapsedMs: number, nowMs: number): number {
  return Math.max(0, Math.ceil(startedAtMs + scheduledElapsedMs - nowMs))
}

export interface FruitTrainRun {
  targetIndex: number
  positions: number[]
  totalMs: number
}

export function createFruitTrainRuns(
  startIndex: number,
  targetIndices: readonly number[],
  cellCount = FRUIT_REFERENCE_CELLS.length,
): FruitTrainRun[] {
  const safeCellCount = Math.max(1, Math.floor(cellCount))
  let current = ((Math.floor(startIndex) % safeCellCount) + safeCellCount) % safeCellCount

  return targetIndices.map((rawTarget, runIndex) => {
    const targetIndex = ((Math.floor(rawTarget) % safeCellCount) + safeCellCount) % safeCellCount
    const laps = runIndex === 0 ? FRUIT_TRAIN_TIMING.firstRunLaps : FRUIT_TRAIN_TIMING.nextRunLaps
    const distance = (targetIndex - current + safeCellCount) % safeCellCount
    const stepCount = laps * safeCellCount + distance
    const positions = Array.from({ length: stepCount }, (_, step) => (current + step + 1) % safeCellCount)
    current = targetIndex
    return {
      targetIndex,
      positions,
      totalMs: runIndex === 0 ? FRUIT_TRAIN_TIMING.firstRunMs : FRUIT_TRAIN_TIMING.nextRunMs,
    }
  })
}

interface LampCell {
  panel: Phaser.GameObjects.Rectangle
  label: Phaser.GameObjects.Text
  icon?: Phaser.GameObjects.Image
  baseFill: number
}

interface CenterLamp {
  panel: Phaser.GameObjects.Arc
  label: Phaser.GameObjects.Text
  icon?: Phaser.GameObjects.Image
  baseFill: number
}

interface BetDoorView {
  panel: Phaser.GameObjects.Rectangle
  value: Phaser.GameObjects.Text
}

export class FruitMachineScene extends ArcadeScene {
  private bets = Array<number>(8).fill(0)
  private lastBets = Array<number>(8).fill(0)
  private betDoors: BetDoorView[] = []
  private lamps: LampCell[] = []
  private centerLamps: CenterLamp[] = []
  private activeLamp = 0
  private activeCenterLamp = 0
  private busy = false
  private betHoldPointerId: number | null = null
  private betHoldDoorIndex: number | null = null
  private betHoldStartTimer: Phaser.Time.TimerEvent | null = null
  private betHoldRepeatTimer: Phaser.Time.TimerEvent | null = null
  private walletText!: Phaser.GameObjects.Text
  private totalText!: Phaser.GameObjects.Text
  private winText!: Phaser.GameObjects.Text
  private messageText!: Phaser.GameObjects.Text
  private multiplierText!: Phaser.GameObjects.Text

  constructor(hooks: SceneHooks) {
    super('merge2048', hooks)
  }

  preload(): void {
    const base = `${import.meta.env.BASE_URL}game-assets/fruit-machine/`
    DOORS.forEach((door) => this.load.image(this.textureKey(door), `${base}${ASSET_FILES[door]}`))
  }

  create(): void {
    this.configureHighDpiCamera()
    this.cameras.main.setBackgroundColor('#16090b')
    this.drawCabinet()
    this.drawOuterRing()
    this.drawCenterWheel()
    this.drawBetDeck()
    this.bindBetHoldReleaseEvents()
    this.applyHighDpiTextResolution()
    this.bindControls((control) => this.handleControl(control))
    this.input.keyboard?.on('keydown', (event: KeyboardEvent) => {
      if (event.code === 'Space') {
        event.preventDefault()
        void this.startSpin()
      } else if (event.key.toLowerCase() === 'c') {
        this.clearBets()
      } else if (event.key.toLowerCase() === 'r') {
        this.repeatBets()
      } else if (event.key.toLowerCase() === 'p') {
        this.handleControl('pause')
      }
    })
    this.begin()
    this.syncWalletState()
    this.refreshBets()
  }

  syncWalletState(): void {
    const state = this.hooks.getFruitState?.()
    this.walletText?.setText(this.formatInteger(state?.credits ?? '0'))
  }

  protected canRestart(): boolean {
    return !this.busy
  }

  private textureKey(door: FruitDoor): string {
    return `fruit-machine-${door}`
  }

  private drawCabinet(): void {
    const frame = this.add.graphics()
    frame.fillStyle(0x6e131a, 1)
    frame.fillRoundedRect(12, 8, 876, 604, 8)
    frame.lineStyle(5, 0xe8b443, 1)
    frame.strokeRoundedRect(16, 12, 868, 596, 7)
    frame.lineStyle(2, 0x145f42, 1)
    frame.strokeRoundedRect(24, 20, 852, 580, 5)
    frame.fillStyle(0x2a0a0e, 1)
    frame.fillRoundedRect(34, 24, 832, 44, 5)
    frame.lineStyle(2, 0xf0c35b, 0.9)
    frame.strokeRoundedRect(34, 24, 832, 44, 5)

    this.add.text(450, 27, '金猪水果机', {
      color: '#fff1a8',
      fontFamily: 'Microsoft YaHei, sans-serif',
      fontSize: '23px',
      fontStyle: 'bold',
      stroke: '#7f151c',
      strokeThickness: 4,
    }).setOrigin(0.5, 0)

    this.add.text(52, 30, '本局赢分', {
      color: '#f5d27a', fontFamily: 'Microsoft YaHei, sans-serif', fontSize: '11px', fontStyle: 'bold',
    })
    this.winText = this.add.text(52, 44, '0', {
      color: '#ffef72', fontFamily: 'ui-monospace, monospace', fontSize: '19px', fontStyle: 'bold',
    })
    this.add.text(733, 30, '积分', {
      color: '#a8e3c0', fontFamily: 'Microsoft YaHei, sans-serif', fontSize: '11px', fontStyle: 'bold',
    })
    this.walletText = this.add.text(848, 42, '0', {
      color: '#b9f3ce', fontFamily: 'ui-monospace, monospace', fontSize: '19px', fontStyle: 'bold',
    }).setOrigin(1, 0.5)
    this.totalText = this.add.text(694, 44, '总注 0', {
      color: '#ffe07a', fontFamily: 'ui-monospace, monospace', fontSize: '14px', fontStyle: 'bold',
    }).setOrigin(1, 0.5)
  }

  private outerPositions(): Array<[number, number, number, number]> {
    const result: Array<[number, number, number, number]> = []
    for (let i = 0; i < 7; i += 1) result.push([151 + i * 99, 99, 92, 54])
    for (let i = 0; i < 5; i += 1) result.push([797, 156 + i * 54, 82, 50])
    for (let i = 0; i < 7; i += 1) result.push([745 - i * 99, 426, 92, 54])
    for (let i = 0; i < 5; i += 1) result.push([103, 372 - i * 54, 82, 50])
    return result
  }

  private drawOuterRing(): void {
    const positions = this.outerPositions()
    this.lamps = positions.map(([x, y, width, height], index) => {
      const cell = FRUIT_REFERENCE_CELLS[index]
      const baseFill = cell.special ? 0x8a3f19 : 0xfff4cf
      const panel = this.add.rectangle(x, y, width, height, baseFill, 1)
        .setStrokeStyle(2, 0xd3a12e, 1)

      let icon: Phaser.GameObjects.Image | undefined
      let label: Phaser.GameObjects.Text
      if (cell.symbol) {
        const iconSize = cell.size === 'small' ? 31 : 37
        icon = this.add.image(x - width * 0.22, y - 1, this.textureKey(cell.symbol))
          .setDisplaySize(iconSize, iconSize)
        label = this.add.text(x + width * 0.2, y, cell.printed, {
          color: '#7f1820',
          fontFamily: 'ui-monospace, Microsoft YaHei, sans-serif',
          fontSize: cell.printed.length >= 5 ? '10px' : '12px',
          fontStyle: 'bold',
          align: 'center',
        }).setOrigin(0.5)
      } else {
        this.add.circle(x, y, 17, 0xf2bc42, 1).setStrokeStyle(2, 0x7e2519, 1)
        label = this.add.text(x, y, '福', {
          color: '#8a171c', fontFamily: 'STKaiti, KaiTi, serif', fontSize: '20px', fontStyle: 'bold',
        }).setOrigin(0.5)
      }
      panel.setDepth(1)
      icon?.setDepth(2)
      label.setDepth(3)
      return { panel, icon, label, baseFill }
    })
    this.setActiveLamp(0)
  }

  private drawCenterWheel(): void {
    const center = this.add.graphics()
    center.fillStyle(0x123d2c, 1)
    center.fillRoundedRect(167, 135, 566, 250, 8)
    center.lineStyle(3, 0xe7b848, 1)
    center.strokeRoundedRect(167, 135, 566, 250, 8)
    center.lineStyle(1, 0xf3df9c, 0.45)
    center.strokeEllipse(450, 260, 426, 198)

    this.centerLamps = CENTER_WHEEL.map((cell, index) => {
      const angle = -Math.PI / 2 + index * (Math.PI * 2 / CENTER_WHEEL.length)
      const x = 450 + Math.cos(angle) * 210
      const y = 260 + Math.sin(angle) * 99
      const baseFill = cell.symbol ? 0xe8e0b9 : (cell.accent ?? 0x4c7d62)
      const panel = this.add.circle(x, y, 22, baseFill, 1).setStrokeStyle(2, 0xd8ac43, 1)
      let icon: Phaser.GameObjects.Image | undefined
      let label: Phaser.GameObjects.Text
      if (cell.symbol) {
        icon = this.add.image(x, y - 1, this.textureKey(cell.symbol)).setDisplaySize(30, 30)
        label = this.add.text(x, y + 15, '', { fontSize: '1px' }).setOrigin(0.5)
      } else {
        label = this.add.text(x, y, cell.label, {
          color: '#fff2b0',
          fontFamily: 'Microsoft YaHei, sans-serif',
          fontSize: cell.label.length >= 4 ? '9px' : '10px',
          fontStyle: 'bold',
          align: 'center',
          lineSpacing: -2,
        }).setOrigin(0.5)
      }
      panel.setDepth(2)
      icon?.setDepth(3)
      label.setDepth(3)
      return { panel, icon, label, baseFill }
    })

    this.add.rectangle(450, 258, 188, 72, 0x24060a, 1).setStrokeStyle(4, 0xf1b93c, 1).setDepth(4)
    this.multiplierText = this.add.text(450, 246, 'READY', {
      color: '#ffdc55', fontFamily: 'ui-monospace, monospace', fontSize: '24px', fontStyle: 'bold',
    }).setOrigin(0.5).setDepth(5)
    this.messageText = this.add.text(450, 277, '等待启动', {
      color: '#ffffff', fontFamily: 'Microsoft YaHei, sans-serif', fontSize: '12px', fontStyle: 'bold',
      align: 'center', wordWrap: { width: 174 },
    }).setOrigin(0.5).setDepth(5)
    this.setActiveCenterLamp(0)
  }

  private drawBetDeck(): void {
    this.betDoors = []
    DOORS.forEach((door, index) => {
      const x = 65 + index * 110
      const panel = this.add.rectangle(x, 503, 100, 78, 0x2c0b0f, 1)
        .setStrokeStyle(2, 0xd7a73e, 1)
        .setInteractive({ useHandCursor: true })
      this.add.image(x - 25, 493, this.textureKey(door)).setDisplaySize(39, 39)
      this.add.text(x + 20, 480, LABELS[door], {
        color: '#ffe9a5', fontFamily: 'Microsoft YaHei, sans-serif', fontSize: '12px', fontStyle: 'bold',
      }).setOrigin(0.5)
      const value = this.add.text(x + 20, 500, '0', {
        color: '#ffffff', fontFamily: 'ui-monospace, monospace', fontSize: '19px', fontStyle: 'bold',
      }).setOrigin(0.5)
      this.add.text(x, 529, BET_COPY[door], {
        color: '#91d9af', fontFamily: 'Microsoft YaHei, sans-serif', fontSize: '9px', fontStyle: 'bold',
      }).setOrigin(0.5)
      panel.on('pointerdown', (pointer: Phaser.Input.Pointer) => this.startBetHold(index, pointer))
      panel.on('pointerup', (pointer: Phaser.Input.Pointer) => this.stopBetHold(pointer))
      panel.on('pointerout', (pointer: Phaser.Input.Pointer) => this.stopBetHold(pointer))
      panel.on('pointerupoutside', (pointer: Phaser.Input.Pointer) => this.stopBetHold(pointer))
      this.betDoors.push({ panel, value })
    })
    this.addActionButton(274, 574, 126, '清注', 0x5e1720, () => this.clearBets())
    this.addActionButton(416, 574, 126, '续押', 0x245f42, () => this.repeatBets())
    this.addActionButton(631, 574, 248, '启动', 0xb7222e, () => void this.startSpin())
  }

  private addActionButton(x: number, y: number, width: number, label: string, color: number, action: () => void): void {
    const button = this.add.rectangle(x, y, width, 42, color, 1)
      .setStrokeStyle(2, 0xffd66d, 1)
      .setInteractive({ useHandCursor: true })
    const text = this.add.text(x, y, label, {
      color: '#fff7d1', fontFamily: 'Microsoft YaHei, sans-serif', fontSize: '16px', fontStyle: 'bold',
    }).setOrigin(0.5)
    button.on('pointerdown', action)
    text.setDepth(button.depth + 1)
  }

  private handleControl(control: GameControl): void {
    if (control === 'pause' || control === 'restart') this.stopBetHold()
    if (this.handleCommonControl(control)) return
    if (control === 'action') void this.startSpin()
  }

  private bindBetHoldReleaseEvents(): void {
    this.input.on('pointerup', this.handleBetPointerUp, this)
    this.input.on('gameout', this.handleBetGameOut, this)
    this.events.once(Phaser.Scenes.Events.SHUTDOWN, this.teardownBetHold, this)
  }

  private handleBetPointerUp(pointer: Phaser.Input.Pointer): void {
    this.stopBetHold(pointer)
  }

  private handleBetGameOut(): void {
    this.stopBetHold()
  }

  private startBetHold(index: number, pointer: Phaser.Input.Pointer): void {
    this.stopBetHold()
    if (!this.addBet(index)) return

    this.betHoldPointerId = pointer.id
    this.betHoldDoorIndex = index
    if (this.bets[index] >= MAX_BET_PER_DOOR) {
      this.stopBetHold()
      return
    }

    const pointerId = pointer.id
    this.betHoldStartTimer = this.time.delayedCall(FRUIT_BET_HOLD_TIMING.initialDelayMs, () => {
      this.betHoldStartTimer = null
      if (!this.isBetHoldActive(index, pointerId)) return

      this.betHoldRepeatTimer = this.time.addEvent({
        delay: FRUIT_BET_HOLD_TIMING.repeatMs,
        loop: true,
        callback: () => {
          if (!this.isBetHoldActive(index, pointerId) || !this.addBet(index)) this.stopBetHold()
        },
      })
    })
  }

  private isBetHoldActive(index: number, pointerId: number): boolean {
    return this.betHoldDoorIndex === index && this.betHoldPointerId === pointerId
  }

  private stopBetHold(pointer?: Phaser.Input.Pointer): void {
    if (pointer && this.betHoldPointerId !== null && pointer.id !== this.betHoldPointerId) return
    this.betHoldPointerId = null
    this.betHoldDoorIndex = null
    this.betHoldStartTimer?.remove(false)
    this.betHoldRepeatTimer?.remove(false)
    this.betHoldStartTimer = null
    this.betHoldRepeatTimer = null
  }

  private teardownBetHold(): void {
    this.stopBetHold()
    this.input.off('pointerup', this.handleBetPointerUp, this)
    this.input.off('gameout', this.handleBetGameOut, this)
  }

  private addBet(index: number): boolean {
    if (this.busy || this.status !== 'playing' || this.bets[index] >= MAX_BET_PER_DOOR) return false
    this.bets[index] += 1
    this.emitSound('move')
    this.refreshBets()
    return true
  }

  private clearBets(): void {
    this.stopBetHold()
    if (this.busy || this.status !== 'playing') return
    this.bets.fill(0)
    this.refreshBets()
  }

  private repeatBets(): void {
    this.stopBetHold()
    if (this.busy || this.status !== 'playing' || this.lastBets.every((value) => value === 0)) return
    this.bets = [...this.lastBets]
    this.refreshBets()
  }

  private refreshBets(): void {
    this.betDoors.forEach((door, index) => door.value.setText(String(this.bets[index])))
    this.totalText?.setText(`总注 ${this.bets.reduce((sum, value) => sum + value, 0)}`)
  }

  private async startSpin(): Promise<void> {
    this.stopBetHold()
    if (this.busy || this.status !== 'playing') return
    const total = this.bets.reduce((sum, value) => sum + value, 0)
    if (total <= 0) {
      this.messageText.setText('请先下注')
      this.emitSound('fruit-no-win')
      return
    }
    const state = this.hooks.getFruitState?.()
    if (!state?.loyaltyEnabled || !state.loyaltyAvailable || !this.hooks.requestFruitSpin) {
      this.messageText.setText('积分服务暂不可用')
      return
    }
    if (BigInt(state.credits) < BigInt(total)) {
      this.messageText.setText('积分不足')
      this.emitSound('fruit-no-win')
      return
    }

    const requestBets = Object.fromEntries(DOORS.map((door, index) => [door, String(this.bets[index])])) as FruitBets
    this.busy = true
    this.lastBets = [...this.bets]
    this.messageText.setText('正在开奖')
    this.multiplierText.setText('----')
    this.winText.setText('0')
    try {
      const result = await this.hooks.requestFruitSpin(requestBets)
      await this.animateCenterPrelude()
      await this.animateTo(result.stop_index)
      await this.animateRewards(result)
      this.hooks.onFruitResult?.(result)
      this.syncWalletState()
      const payout = this.safeNumber(result.payout)
      this.setScore(payout)
      this.showSettlement(result)
    } catch (error: unknown) {
      this.restoreRetryBets(error)
      this.messageText.setText(this.hooks.formatSlotError?.(error) ?? '开奖失败，请稍后重试')
      this.multiplierText.setText('ERROR')
      this.emitSound('fruit-no-win')
      this.hooks.onFruitError?.(error)
    } finally {
      this.busy = false
    }
  }

  private restoreRetryBets(error: unknown): void {
    const pending = error && typeof error === 'object'
      ? (error as { fruitRetryBets?: unknown }).fruitRetryBets
      : undefined
    if (pending && typeof pending === 'object') {
      const values = DOORS.map((door) => Number((pending as Record<string, unknown>)[door]))
      if (values.every((value) => Number.isSafeInteger(value) && value >= 0 && value <= 100)) {
        this.bets = values
        this.lastBets = [...values]
        this.refreshBets()
        return
      }
    }
    this.bets = [...this.lastBets]
    this.refreshBets()
  }

  private async animateTo(stopIndex: number): Promise<void> {
    const target = Math.max(0, Math.min(FRUIT_REFERENCE_CELLS.length - 1, Math.floor(stopIndex)))
    this.messageText.setText('外圈跑灯')
    if (this.hooks.prefersReducedMotion?.()) {
      this.setActiveLamp(target)
      this.showOuterStop(target)
      this.emitSound('fruit-reel-stop')
      await this.delay(100)
      return
    }
    const startedAtMs = await this.startSynchronizedTrack('fruit-outer-spin')
    const distance = (target - this.activeLamp + FRUIT_REFERENCE_CELLS.length) % FRUIT_REFERENCE_CELLS.length
    const steps = FRUIT_ANIMATION_TIMING.outerLaps * FRUIT_REFERENCE_CELLS.length + distance
    const delays = createFruitRunDelays(steps, FRUIT_ANIMATION_TIMING.outerRunMs)
    await this.runTimedSteps(delays, startedAtMs, () => {
      this.setActiveLamp((this.activeLamp + 1) % FRUIT_REFERENCE_CELLS.length)
    })
    this.showOuterStop(target)
    await this.flashOuterTarget(target, 2)
  }

  private async animateCenterPrelude(): Promise<void> {
    this.messageText.setText('中央灯盘')
    if (this.hooks.prefersReducedMotion?.()) {
      this.multiplierText.setText('----')
      await this.delay(100)
      return
    }

    const startedAtMs = await this.startSynchronizedTrack('fruit-center-spin')
    const delays = Array<number>(FRUIT_ANIMATION_TIMING.centerPreludeSteps)
      .fill(FRUIT_ANIMATION_TIMING.centerPreludeStepMs)
    await this.runTimedSteps(delays, startedAtMs, (step) => {
      this.setCenterWarmup(step)
      if (step % 6 === 0) {
        this.multiplierText.setText(CENTER_DISPLAY_VALUES[Math.floor(step / 6) % CENTER_DISPLAY_VALUES.length])
      }
    })
    this.setActiveCenterLamp(this.activeCenterLamp)
  }

  private async animateRewards(result: FruitSpinServerResult): Promise<void> {
    if (result.outcome.kind !== 'fruit') {
      const centerTarget = Math.max(0, Math.min(15, Math.floor(result.outcome.center_index ?? this.centerIndexFor(result))))
      await this.animateCenterTo(centerTarget)
    }
    const stops: FruitRewardStop[] = result.outcome.kind === 'send_light'
      ? result.outcome.extra_stops ?? []
      : result.outcome.kind === 'little_mary' ? result.outcome.little_mary_stops ?? [] : []
    if (stops.length > 0) this.emitSound('fruit-bonus-enter')

    if (result.outcome.kind === 'send_light' && stops.length > 0 && !this.hooks.prefersReducedMotion?.()) {
      await this.animateTrainRewards(stops)
      return
    }

    for (const stop of stops) {
      const index = Math.max(0, Math.min(FRUIT_REFERENCE_CELLS.length - 1, Math.floor(stop.index)))
      this.setActiveLamp(index)
      this.multiplierText.setText(`×${stop.multiplier}`)
      this.emitSound('fruit-bonus-land')
      if (!this.hooks.prefersReducedMotion?.()) await this.flashOuterTarget(index, 1)
      await this.delay(this.hooks.prefersReducedMotion?.() ? 80 : 260)
    }
  }

  private async animateTrainRewards(stops: readonly FruitRewardStop[]): Promise<void> {
    const targetIndices = stops.map((stop) => (
      Math.max(0, Math.min(FRUIT_REFERENCE_CELLS.length - 1, Math.floor(stop.index)))
    ))
    const runs = createFruitTrainRuns(this.activeLamp, targetIndices)

    for (let runIndex = 0; runIndex < runs.length; runIndex += 1) {
      const run = runs[runIndex]
      this.messageText.setText(`火车行进 ${runIndex + 1}/${runs.length}`)
      const delays = createFruitRunDelays(run.positions.length, run.totalMs)
      const startedAtMs = this.animationNow()
      await this.runTimedSteps(delays, startedAtMs, (step) => {
        this.setTrainLamp(run.positions[step])
      })

      const stop = stops[runIndex]
      this.setActiveLamp(run.targetIndex)
      this.multiplierText.setText(`×${stop.multiplier}`)
      this.messageText.setText(`火车奖励 ${runIndex + 1}/${runs.length}`)
      this.emitSound('fruit-bonus-land')
      await this.flashOuterTarget(run.targetIndex, 1)
      await this.delay(FRUIT_TRAIN_TIMING.rewardHoldMs)
    }
  }

  private centerIndexFor(result: FruitSpinServerResult): number {
    if (result.outcome.kind === 'send_light') return 14
    if (result.outcome.kind === 'little_mary') return 10
    if (result.outcome.kind === 'jackpot') return 4
    return 0
  }

  private async animateCenterTo(target: number): Promise<void> {
    this.messageText.setText('中央奖盘')
    if (this.hooks.prefersReducedMotion?.()) {
      this.setActiveCenterLamp(target)
      this.multiplierText.setText(CENTER_WHEEL[target].label.replace('\n', ''))
      this.emitSound('fruit-center-stop')
      await this.delay(100)
      return
    }
    const startedAtMs = await this.startSynchronizedTrack('fruit-center-spin')
    const distance = (target - this.activeCenterLamp + CENTER_WHEEL.length) % CENTER_WHEEL.length
    const steps = FRUIT_ANIMATION_TIMING.centerLaps * CENTER_WHEEL.length + distance
    const delays = createFruitRunDelays(steps, FRUIT_ANIMATION_TIMING.centerRunMs)
    await this.runTimedSteps(delays, startedAtMs, (step) => {
      this.setActiveCenterLamp((this.activeCenterLamp + 1) % CENTER_WHEEL.length)
      if (step % 6 === 0) {
        this.multiplierText.setText(CENTER_DISPLAY_VALUES[Math.floor(step / 6) % CENTER_DISPLAY_VALUES.length])
      }
    })
    this.multiplierText.setText(CENTER_WHEEL[target].label.replace('\n', ''))
    await this.flashCenterTarget(target, 2)
    this.emitSound('fruit-center-stop')
  }

  private showSettlement(result: FruitSpinServerResult): void {
    const payout = this.safeNumber(result.payout)
    this.winText.setText(this.formatInteger(result.payout))
    if (result.outcome.kind === 'jackpot') {
      const tier = { bronze: '铜彩金', silver: '银彩金', gold: '金彩金' }[result.outcome.jackpot_tier ?? 'bronze']
      this.multiplierText.setText(`×${result.outcome.jackpot_multiplier ?? '0'}`)
      this.messageText.setText(`${tier} ${result.payout} 分`)
      this.emitSound('fruit-jackpot')
      this.coinCelebration(80)
    } else if (result.outcome.kind === 'little_mary') {
      this.multiplierText.setText('小三元')
      this.messageText.setText(`奖励 ${result.payout} 分`)
      this.emitSound(payout > 0 ? 'fruit-win' : 'fruit-no-win')
      if (payout > 0) this.coinCelebration(40)
    } else if (result.outcome.kind === 'send_light') {
      this.multiplierText.setText('开火车')
      this.messageText.setText(`奖励 ${result.payout} 分`)
      this.emitSound(payout > 0 ? 'fruit-win' : 'fruit-no-win')
      if (payout > 0) this.coinCelebration(28)
    } else {
      const symbol = result.outcome.symbol ? LABELS[result.outcome.symbol] : '水果'
      this.multiplierText.setText(`×${result.outcome.multiplier ?? '0'}`)
      this.messageText.setText(payout > 0 ? `${symbol}中奖 ${result.payout} 分` : `${symbol}未下注`)
      this.emitSound(payout > 0 ? 'fruit-win' : 'fruit-no-win')
      if (payout > 0) this.coinCelebration(18)
    }
  }

  private coinCelebration(count: number): void {
    if (this.hooks.prefersReducedMotion?.()) return
    for (let i = 0; i < count; i += 1) {
      const coin = this.add.circle(Phaser.Math.Between(185, 715), Phaser.Math.Between(150, 300), Phaser.Math.Between(3, 7), 0xffd34f, 0.95)
        .setStrokeStyle(1, 0xfff2a8, 1).setDepth(20)
      this.tweens.add({
        targets: coin,
        x: coin.x + Phaser.Math.Between(-130, 130),
        y: 438,
        alpha: 0,
        duration: Phaser.Math.Between(700, 1300),
        ease: 'Quad.easeIn',
        onComplete: () => coin.destroy(),
      })
    }
  }

  private setActiveLamp(index: number): void {
    this.activeLamp = index
    this.lamps.forEach((lamp, cellIndex) => {
      const active = cellIndex === index
      lamp.panel.setFillStyle(active ? 0x26754f : lamp.baseFill, 1)
      lamp.panel.setStrokeStyle(active ? 4 : 2, active ? 0xffef75 : 0xd3a12e, 1)
      lamp.label.setColor(active ? '#fff5b5' : (FRUIT_REFERENCE_CELLS[cellIndex].special ? '#8a171c' : '#7f1820'))
      if (lamp.icon) lamp.icon.setAlpha(active ? 1 : 0.94)
    })
  }

  private setTrainLamp(headIndex: number): void {
    const trainColors = [0xffd447, 0xd73332, 0xf08a2f, 0x36a764, 0x257a5a] as const
    this.activeLamp = headIndex
    this.lamps.forEach((lamp, cellIndex) => {
      const trailOffset = (headIndex - cellIndex + this.lamps.length) % this.lamps.length
      const trainColor = trailOffset < FRUIT_TRAIN_TIMING.trailLength ? trainColors[trailOffset] : undefined
      const isHead = trailOffset === 0
      lamp.panel.setFillStyle(trainColor ?? lamp.baseFill, 1)
      lamp.panel.setStrokeStyle(trainColor === undefined ? 2 : (isHead ? 4 : 3), isHead ? 0xffffff : (trainColor ?? 0xd3a12e), 1)
      lamp.label.setColor(trainColor === undefined
        ? (FRUIT_REFERENCE_CELLS[cellIndex].special ? '#8a171c' : '#7f1820')
        : (isHead ? '#6b150d' : '#fff7cc'))
      lamp.icon?.setAlpha(trainColor === undefined ? 0.82 : 1)
    })
  }

  private showOuterStop(index: number): void {
    const cell = FRUIT_REFERENCE_CELLS[index]
    this.multiplierText.setText(cell.special ? 'BONUS' : `×${cell.multiplier ?? 0}`)
  }

  private async flashOuterTarget(index: number, cycles: number): Promise<void> {
    const lamp = this.lamps[index]
    if (!lamp) return
    for (let cycle = 0; cycle < cycles; cycle += 1) {
      lamp.panel.setFillStyle(0xffcf42, 1)
      lamp.panel.setStrokeStyle(4, 0xffffff, 1)
      lamp.label.setColor('#7f1820')
      await this.delay(FRUIT_ANIMATION_TIMING.landingFlashMs)
      this.setActiveLamp(index)
      await this.delay(FRUIT_ANIMATION_TIMING.landingFlashMs)
    }
  }

  private setCenterWarmup(step: number): void {
    this.centerLamps.forEach((lamp, index) => {
      const active = (index - step + CENTER_WHEEL.length * 4) % 4 === 0
      const color = CENTER_PULSE_COLORS[(index + Math.floor(step / 4)) % CENTER_PULSE_COLORS.length]
      lamp.panel.setFillStyle(active ? color : lamp.baseFill, 1)
      lamp.panel.setStrokeStyle(active ? 4 : 2, active ? 0xfff6a5 : 0xd8ac43, 1)
      lamp.label.setColor(active ? '#fffbd2' : '#fff2b0')
      lamp.icon?.setAlpha(active ? 1 : 0.78)
    })
  }

  private async flashCenterTarget(index: number, cycles: number): Promise<void> {
    const lamp = this.centerLamps[index]
    if (!lamp) return
    for (let cycle = 0; cycle < cycles; cycle += 1) {
      lamp.panel.setFillStyle(0xffcf42, 1)
      lamp.panel.setStrokeStyle(4, 0xffffff, 1)
      await this.delay(FRUIT_ANIMATION_TIMING.landingFlashMs)
      this.setActiveCenterLamp(index)
      await this.delay(FRUIT_ANIMATION_TIMING.landingFlashMs)
    }
  }

  private setActiveCenterLamp(index: number): void {
    this.activeCenterLamp = index
    this.centerLamps.forEach((lamp, cellIndex) => {
      const active = cellIndex === index
      lamp.panel.setFillStyle(active ? 0x2c9a62 : lamp.baseFill, 1)
      lamp.panel.setStrokeStyle(active ? 4 : 2, active ? 0xffef75 : 0xd8ac43, 1)
      lamp.label.setColor(active ? '#fff8c7' : '#fff2b0')
      lamp.icon?.setAlpha(active ? 1 : 0.92)
    })
  }

  private configureHighDpiCamera(): void {
    const renderScale = Math.max(1, this.hooks.renderScale ?? 1)
    if (renderScale === 1) return
    this.cameras.main.setZoom(renderScale)
    this.cameras.main.centerOn(GAME_WIDTH / 2, GAME_HEIGHT / 2)
  }

  private applyHighDpiTextResolution(): void {
    const renderScale = Math.max(1, this.hooks.renderScale ?? 1)
    if (renderScale === 1) return
    this.children.list.forEach((child) => {
      if (child instanceof Phaser.GameObjects.Text) child.setResolution(renderScale)
    })
  }

  private async startSynchronizedTrack(sound: 'fruit-center-spin' | 'fruit-outer-spin'): Promise<number> {
    if (this.hooks.playSynchronizedSound) {
      const anchor = await this.hooks.playSynchronizedSound(sound)
      return anchor.startedAtMs
    }
    this.emitSound(sound)
    return this.animationNow()
  }

  private async runTimedSteps(
    delays: readonly number[],
    startedAtMs: number,
    advance: (step: number) => void,
  ): Promise<void> {
    let scheduledElapsedMs = 0
    for (let step = 0; step < delays.length; step += 1) {
      advance(step)
      scheduledElapsedMs += delays[step]
      const waitMs = fruitTimelineWaitMs(startedAtMs, scheduledElapsedMs, this.animationNow())
      if (waitMs > 0) await this.delay(waitMs)
    }
  }

  private animationNow(): number {
    return typeof performance === 'undefined' ? Date.now() : performance.now()
  }

  private delay(ms: number): Promise<void> {
    return new Promise((resolve) => this.time.delayedCall(ms, resolve))
  }

  private safeNumber(value: string): number {
    if (!/^\d+$/.test(value)) return 0
    return Math.min(1_000_000_000, Number(value))
  }

  private formatInteger(value: string): string {
    if (!/^\d+$/.test(value)) return '--'
    try {
      return new Intl.NumberFormat('zh-CN').format(BigInt(value))
    } catch {
      return '--'
    }
  }
}
