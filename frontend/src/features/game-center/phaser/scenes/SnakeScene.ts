import Phaser from 'phaser'
import { ArcadeScene } from '../ArcadeScene'
import type { GameControl, SceneHooks } from '../../types'

interface Cell {
  x: number
  y: number
}

const COLS = 20
const ROWS = 20
const CELL_SIZE = 24
const BOARD_X = 120
const BOARD_Y = 88

export class SnakeScene extends ArcadeScene {
  private snake: Cell[] = []
  private food: Cell = { x: 15, y: 10 }
  private direction: Cell = { x: 1, y: 0 }
  private queuedDirection: Cell = { x: 1, y: 0 }
  private graphics!: Phaser.GameObjects.Graphics
  private hud!: Phaser.GameObjects.Text
  private tickTimer?: Phaser.Time.TimerEvent
  private swipeStart?: Phaser.Math.Vector2

  constructor(hooks: SceneHooks) {
    super('snake', hooks)
  }

  create(): void {
    this.tickTimer = undefined
    this.swipeStart = undefined

    this.drawFrame('SNAKE // GRID RUNNER', 0x4ade80)
    this.graphics = this.add.graphics()
    this.hud = this.addHud('SCORE  0000\nLENGTH 0003')
    this.add.text(710, 176, 'ARROWS / WASD\nSPACE: PAUSE\nR: RESTART', {
      color: '#64748b',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '14px',
      lineSpacing: 8,
    })

    this.snake = [
      { x: 6, y: 10 },
      { x: 5, y: 10 },
      { x: 4, y: 10 },
    ]
    this.direction = { x: 1, y: 0 }
    this.queuedDirection = { x: 1, y: 0 }
    this.spawnFood()
    this.begin()
    this.renderBoard()

    this.bindControls((control) => this.handleControl(control))
    this.input.keyboard?.on('keydown', (event: KeyboardEvent) => {
      const keyMap: Record<string, GameControl> = {
        ArrowLeft: 'left',
        a: 'left',
        A: 'left',
        ArrowRight: 'right',
        d: 'right',
        D: 'right',
        ArrowUp: 'up',
        w: 'up',
        W: 'up',
        ArrowDown: 'down',
        s: 'down',
        S: 'down',
        ' ': 'pause',
        r: 'restart',
        R: 'restart',
      }
      const control = keyMap[event.key]
      if (control) this.handleControl(control)
    })

    this.input.on('pointerdown', (pointer: Phaser.Input.Pointer) => {
      this.swipeStart = new Phaser.Math.Vector2(pointer.x, pointer.y)
    })
    this.input.on('pointerup', (pointer: Phaser.Input.Pointer) => {
      if (!this.swipeStart) return
      const dx = pointer.x - this.swipeStart.x
      const dy = pointer.y - this.swipeStart.y
      this.swipeStart = undefined
      if (Math.max(Math.abs(dx), Math.abs(dy)) < 24) return
      this.handleControl(Math.abs(dx) > Math.abs(dy) ? (dx > 0 ? 'right' : 'left') : (dy > 0 ? 'down' : 'up'))
    })

    this.tickTimer = this.time.addEvent({
      delay: 125,
      loop: true,
      callback: () => this.tick(),
    })
  }

  private handleControl(control: GameControl): void {
    if (this.handleCommonControl(control)) return
    const vectors: Partial<Record<GameControl, Cell>> = {
      left: { x: -1, y: 0 },
      right: { x: 1, y: 0 },
      up: { x: 0, y: -1 },
      down: { x: 0, y: 1 },
    }
    const next = vectors[control]
    if (!next || this.status !== 'playing') return
    if (next.x + this.direction.x === 0 && next.y + this.direction.y === 0) return
    if (next.x === this.queuedDirection.x && next.y === this.queuedDirection.y) return
    this.queuedDirection = next
    this.emitSound('move')
  }

  private tick(): void {
    if (this.status !== 'playing') return
    this.direction = this.queuedDirection
    const head = this.snake[0]
    const next = {
      x: head.x + this.direction.x,
      y: head.y + this.direction.y,
    }
    const hitWall = next.x < 0 || next.x >= COLS || next.y < 0 || next.y >= ROWS
    const willGrow = next.x === this.food.x && next.y === this.food.y
    const occupiedCells = willGrow ? this.snake : this.snake.slice(0, -1)
    const hitSelf = occupiedCells.some((cell) => cell.x === next.x && cell.y === next.y)
    if (hitWall || hitSelf) {
      this.tickTimer?.remove(false)
      this.finish()
      this.add.text(BOARD_X + 240, BOARD_Y + 240, 'GAME OVER', {
        color: '#f43f5e',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: '32px',
        fontStyle: 'bold',
      }).setOrigin(0.5).setDepth(3)
      return
    }

    this.snake.unshift(next)
    if (willGrow) {
      this.emitSound('collect')
      this.addScore(10)
      if (!this.spawnFood()) {
        this.tickTimer?.remove(false)
        this.finish('complete')
        this.renderBoard()
        this.add.text(BOARD_X + 240, BOARD_Y + 240, 'GRID CLEARED', {
          color: '#4ade80',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
          fontSize: '30px',
          fontStyle: 'bold',
        }).setOrigin(0.5).setDepth(3)
        return
      }
    } else {
      this.snake.pop()
    }
    this.renderBoard()
  }

  private spawnFood(): boolean {
    const free: Cell[] = []
    for (let y = 0; y < ROWS; y += 1) {
      for (let x = 0; x < COLS; x += 1) {
        if (!this.snake.some((cell) => cell.x === x && cell.y === y)) free.push({ x, y })
      }
    }
    const food = Phaser.Utils.Array.GetRandom(free)
    if (!food) return false
    this.food = food
    return true
  }

  private renderBoard(): void {
    this.graphics.clear()
    this.graphics.fillStyle(0x07130f, 1)
    this.graphics.fillRoundedRect(BOARD_X - 8, BOARD_Y - 8, COLS * CELL_SIZE + 16, ROWS * CELL_SIZE + 16, 8)
    this.graphics.lineStyle(1, 0x22c55e, 0.12)
    for (let x = 0; x <= COLS; x += 1) {
      this.graphics.lineBetween(BOARD_X + x * CELL_SIZE, BOARD_Y, BOARD_X + x * CELL_SIZE, BOARD_Y + ROWS * CELL_SIZE)
    }
    for (let y = 0; y <= ROWS; y += 1) {
      this.graphics.lineBetween(BOARD_X, BOARD_Y + y * CELL_SIZE, BOARD_X + COLS * CELL_SIZE, BOARD_Y + y * CELL_SIZE)
    }
    this.snake.forEach((cell, index) => {
      this.graphics.fillStyle(index === 0 ? 0x86efac : 0x22c55e, 1)
      this.graphics.fillRoundedRect(
        BOARD_X + cell.x * CELL_SIZE + 2,
        BOARD_Y + cell.y * CELL_SIZE + 2,
        CELL_SIZE - 4,
        CELL_SIZE - 4,
        index === 0 ? 7 : 4,
      )
    })
    this.graphics.fillStyle(0xf43f5e, 1)
    this.graphics.fillCircle(
      BOARD_X + this.food.x * CELL_SIZE + CELL_SIZE / 2,
      BOARD_Y + this.food.y * CELL_SIZE + CELL_SIZE / 2,
      CELL_SIZE * 0.36,
    )
    this.hud.setText(`SCORE  ${String(this.score).padStart(4, '0')}\nLENGTH ${String(this.snake.length).padStart(4, '0')}`)
  }
}
