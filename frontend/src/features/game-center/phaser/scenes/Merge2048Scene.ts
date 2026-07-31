import Phaser from 'phaser'
import { ArcadeScene } from '../ArcadeScene'
import { canMove2048, move2048, type Grid2048 } from '../merge2048'
import type { GameControl, SceneHooks } from '../../types'

const SIZE = 4
const CELL = 108
const GAP = 10
const BOARD_X = 154
const BOARD_Y = 92

const TILE_COLORS: Record<number, number> = {
  0: 0x172033,
  2: 0x164e63,
  4: 0x155e75,
  8: 0x0e7490,
  16: 0x0f766e,
  32: 0x15803d,
  64: 0x65a30d,
  128: 0xca8a04,
  256: 0xd97706,
  512: 0xea580c,
  1024: 0xe11d48,
  2048: 0xc026d3,
}

export class Merge2048Scene extends ArcadeScene {
  private grid: Grid2048 = []
  private graphics!: Phaser.GameObjects.Graphics
  private tileTexts: Phaser.GameObjects.Text[] = []
  private hud!: Phaser.GameObjects.Text
  private swipeStart?: Phaser.Math.Vector2

  constructor(hooks: SceneHooks) {
    super('merge2048', hooks)
  }

  create(): void {
    this.tileTexts = []
    this.swipeStart = undefined

    this.drawFrame('2048 // MERGE MATRIX', 0xfbbf24)
    this.graphics = this.add.graphics()
    this.hud = this.addHud('SCORE 00000\nMAX   00002', 650, 120)
    this.add.text(650, 236, 'ARROWS / WASD\nSWIPE TO MOVE\nP: PAUSE\nR: RESTART', {
      color: '#64748b',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '14px',
      lineSpacing: 8,
    })
    this.grid = Array.from({ length: SIZE }, () => Array(SIZE).fill(0))
    this.addRandomTile()
    this.addRandomTile()
    this.begin()
    this.renderGrid()
    this.bindControls((control) => this.handleControl(control))

    this.input.keyboard?.on('keydown', (event: KeyboardEvent) => {
      const keyMap: Record<string, GameControl> = {
        ArrowLeft: 'left', a: 'left', A: 'left',
        ArrowRight: 'right', d: 'right', D: 'right',
        ArrowUp: 'up', w: 'up', W: 'up',
        ArrowDown: 'down', s: 'down', S: 'down',
        p: 'pause', P: 'pause', r: 'restart', R: 'restart',
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
      if (Math.max(Math.abs(dx), Math.abs(dy)) < 28) return
      this.handleControl(Math.abs(dx) > Math.abs(dy) ? (dx > 0 ? 'right' : 'left') : (dy > 0 ? 'down' : 'up'))
    })
  }

  private handleControl(control: GameControl): void {
    if (this.handleCommonControl(control)) return
    if (this.status !== 'playing' || !['left', 'right', 'up', 'down'].includes(control)) return
    const result = move2048(this.grid, control as 'left' | 'right' | 'up' | 'down')
    if (!result.moved) {
      this.finishIfBlocked()
      return
    }
    this.grid = result.grid
    this.emitSound('move')
    if (result.score > 0) this.emitSound('collect')
    this.addScore(result.score)
    this.addRandomTile()
    this.renderGrid()
    this.finishIfBlocked()
  }

  private addRandomTile(): void {
    const empty: Array<{ x: number; y: number }> = []
    this.grid.forEach((row, y) => row.forEach((value, x) => {
      if (!value) empty.push({ x, y })
    }))
    const cell = Phaser.Utils.Array.GetRandom(empty)
    if (cell) this.grid[cell.y][cell.x] = Math.random() < 0.9 ? 2 : 4
  }

  private finishIfBlocked(): void {
    if (canMove2048(this.grid)) return
    this.finish()
    this.add.text(BOARD_X + 226, BOARD_Y + 226, 'NO MOVES', {
      color: '#fb7185',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '32px',
      fontStyle: 'bold',
    }).setOrigin(0.5).setDepth(5)
  }

  private renderGrid(): void {
    this.graphics.clear()
    this.tileTexts.forEach((text) => text.destroy())
    this.tileTexts = []
    this.graphics.fillStyle(0x0b1220, 1)
    this.graphics.fillRoundedRect(BOARD_X - 12, BOARD_Y - 12, SIZE * CELL + (SIZE - 1) * GAP + 24, SIZE * CELL + (SIZE - 1) * GAP + 24, 10)
    let max = 0
    this.grid.forEach((row, y) => row.forEach((value, x) => {
      max = Math.max(max, value)
      const px = BOARD_X + x * (CELL + GAP)
      const py = BOARD_Y + y * (CELL + GAP)
      const color = TILE_COLORS[value] ?? 0x7e22ce
      this.graphics.fillStyle(color, 1)
      this.graphics.fillRoundedRect(px, py, CELL, CELL, 8)
      this.graphics.lineStyle(1, 0xffffff, value ? 0.3 : 0.08)
      this.graphics.strokeRoundedRect(px, py, CELL, CELL, 8)
      if (value) {
        const fontSize = value >= 1024 ? 28 : value >= 128 ? 34 : 40
        this.tileTexts.push(this.add.text(px + CELL / 2, py + CELL / 2, String(value), {
          color: '#f8fafc',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
          fontSize: `${fontSize}px`,
          fontStyle: 'bold',
        }).setOrigin(0.5))
      }
    }))
    this.hud.setText(`SCORE ${String(this.score).padStart(5, '0')}\nMAX   ${String(max).padStart(5, '0')}`)
  }
}
