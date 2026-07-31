import Phaser from 'phaser'
import { ArcadeScene } from '../ArcadeScene'
import type { GameControl, SceneHooks } from '../../types'

type Matrix = number[][]

interface Piece {
  matrix: Matrix
  x: number
  y: number
  color: number
}

const BOARD_COLS = 10
const BOARD_ROWS = 20
const CELL = 25
const BOARD_X = 260
const BOARD_Y = 72

const SHAPES: Array<{ matrix: Matrix; color: number }> = [
  { matrix: [[1, 1, 1, 1]], color: 0x22d3ee },
  { matrix: [[1, 1], [1, 1]], color: 0xfbbf24 },
  { matrix: [[0, 1, 0], [1, 1, 1]], color: 0xc084fc },
  { matrix: [[0, 1, 1], [1, 1, 0]], color: 0x4ade80 },
  { matrix: [[1, 1, 0], [0, 1, 1]], color: 0xf87171 },
  { matrix: [[1, 0, 0], [1, 1, 1]], color: 0x60a5fa },
  { matrix: [[0, 0, 1], [1, 1, 1]], color: 0xfb923c },
]

export class TetrisScene extends ArcadeScene {
  private board: Matrix = []
  private colors: Matrix = []
  private piece!: Piece
  private nextPiece!: Piece
  private graphics!: Phaser.GameObjects.Graphics
  private hud!: Phaser.GameObjects.Text
  private level = 1
  private lines = 0
  private dropTimer?: Phaser.Time.TimerEvent

  constructor(hooks: SceneHooks) {
    super('tetris', hooks)
  }

  create(): void {
    this.dropTimer = undefined

    this.drawFrame('TETRIS // STACK PROTOCOL', 0x22d3ee)
    this.graphics = this.add.graphics()
    this.hud = this.addHud('SCORE 00000\nLINES 000\nLEVEL 01', 570, 105)
    this.add.text(570, 260, 'NEXT', {
      color: '#22d3ee',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '16px',
      fontStyle: 'bold',
    })
    this.add.text(86, 154, 'ARROWS / WASD\nUP: ROTATE\nSPACE: DROP\nP: PAUSE\nR: RESTART', {
      color: '#64748b',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '14px',
      lineSpacing: 8,
    })

    this.board = Array.from({ length: BOARD_ROWS }, () => Array(BOARD_COLS).fill(0))
    this.colors = Array.from({ length: BOARD_ROWS }, () => Array(BOARD_COLS).fill(0))
    this.level = 1
    this.lines = 0
    this.nextPiece = this.randomPiece()
    this.spawnPiece()
    this.begin()
    this.renderGame()
    this.scheduleDrop()
    this.bindControls((control) => this.handleControl(control))

    this.input.keyboard?.on('keydown', (event: KeyboardEvent) => {
      const keyMap: Record<string, GameControl> = {
        ArrowLeft: 'left',
        a: 'left',
        A: 'left',
        ArrowRight: 'right',
        d: 'right',
        D: 'right',
        ArrowDown: 'down',
        s: 'down',
        S: 'down',
        ArrowUp: 'rotate',
        w: 'rotate',
        W: 'rotate',
        ' ': 'drop',
        p: 'pause',
        P: 'pause',
        r: 'restart',
        R: 'restart',
      }
      const control = keyMap[event.key]
      if (control) this.handleControl(control)
    })
  }

  private handleControl(control: GameControl): void {
    if (this.handleCommonControl(control)) return
    if (this.status !== 'playing') return
    if (control === 'left' && this.tryMove(-1, 0)) this.emitSound('move')
    if (control === 'right' && this.tryMove(1, 0)) this.emitSound('move')
    if (control === 'down') {
      if (this.tryMove(0, 1)) {
        this.emitSound('move')
        this.addScore(1)
      }
      else this.lockPiece()
    }
    if (control === 'rotate' || control === 'up') this.rotatePiece()
    if (control === 'drop' || control === 'action') this.hardDrop()
    this.renderGame()
  }

  private scheduleDrop(): void {
    this.dropTimer?.remove(false)
    this.dropTimer = this.time.addEvent({
      delay: Math.max(130, 720 - (this.level - 1) * 58),
      loop: true,
      callback: () => {
        if (!this.tryMove(0, 1)) this.lockPiece()
        this.renderGame()
      },
    })
  }

  private randomPiece(): Piece {
    const shape = Phaser.Utils.Array.GetRandom(SHAPES) ?? SHAPES[0]
    return {
      matrix: shape.matrix.map((row) => [...row]),
      color: shape.color,
      x: 0,
      y: 0,
    }
  }

  private spawnPiece(): void {
    this.piece = this.nextPiece
    this.nextPiece = this.randomPiece()
    this.piece.x = Math.floor((BOARD_COLS - this.piece.matrix[0].length) / 2)
    this.piece.y = 0
    if (this.collides(this.piece.matrix, this.piece.x, this.piece.y)) {
      this.dropTimer?.remove(false)
      this.finish()
    }
  }

  private collides(matrix: Matrix, targetX: number, targetY: number): boolean {
    return matrix.some((row, rowIndex) => row.some((value, colIndex) => {
      if (!value) return false
      const x = targetX + colIndex
      const y = targetY + rowIndex
      return x < 0 || x >= BOARD_COLS || y >= BOARD_ROWS || (y >= 0 && this.board[y][x] !== 0)
    }))
  }

  private tryMove(dx: number, dy: number): boolean {
    if (this.collides(this.piece.matrix, this.piece.x + dx, this.piece.y + dy)) return false
    this.piece.x += dx
    this.piece.y += dy
    return true
  }

  private rotatePiece(): void {
    const rotated = this.piece.matrix[0].map((_, index) => this.piece.matrix.map((row) => row[index]).reverse())
    const kicks = [0, -1, 1, -2, 2]
    const kick = kicks.find((offset) => !this.collides(rotated, this.piece.x + offset, this.piece.y))
    if (kick === undefined) return
    this.piece.matrix = rotated
    this.piece.x += kick
    this.emitSound('tetris-rotate')
  }

  private hardDrop(): void {
    let distance = 0
    while (this.tryMove(0, 1)) distance += 1
    this.addScore(distance * 2)
    this.emitSound('tetris-drop')
    this.lockPiece()
  }

  private lockPiece(): void {
    this.piece.matrix.forEach((row, rowIndex) => {
      row.forEach((value, colIndex) => {
        if (!value) return
        const y = this.piece.y + rowIndex
        const x = this.piece.x + colIndex
        if (y >= 0) {
          this.board[y][x] = 1
          this.colors[y][x] = this.piece.color
        }
      })
    })
    this.clearLines()
    this.spawnPiece()
  }

  private clearLines(): void {
    let cleared = 0
    for (let row = BOARD_ROWS - 1; row >= 0; row -= 1) {
      if (this.board[row].every(Boolean)) {
        this.board.splice(row, 1)
        this.colors.splice(row, 1)
        this.board.unshift(Array(BOARD_COLS).fill(0))
        this.colors.unshift(Array(BOARD_COLS).fill(0))
        cleared += 1
        row += 1
      }
    }
    if (!cleared) return
    this.emitSound('tetris-line-clear')
    const lineScores = [0, 100, 300, 500, 800]
    this.lines += cleared
    this.addScore(lineScores[cleared] * this.level)
    const nextLevel = 1 + Math.floor(this.lines / 10)
    if (nextLevel !== this.level) {
      this.level = nextLevel
      this.scheduleDrop()
    }
  }

  private renderGame(): void {
    this.graphics.clear()
    this.graphics.fillStyle(0x07111a, 1)
    this.graphics.fillRoundedRect(BOARD_X - 8, BOARD_Y - 8, BOARD_COLS * CELL + 16, BOARD_ROWS * CELL + 16, 8)

    const drawBlock = (x: number, y: number, color: number, alpha = 1) => {
      this.graphics.fillStyle(color, alpha)
      this.graphics.fillRoundedRect(BOARD_X + x * CELL + 2, BOARD_Y + y * CELL + 2, CELL - 4, CELL - 4, 4)
      this.graphics.lineStyle(1, 0xffffff, alpha * 0.32)
      this.graphics.strokeRoundedRect(BOARD_X + x * CELL + 2, BOARD_Y + y * CELL + 2, CELL - 4, CELL - 4, 4)
    }

    this.board.forEach((row, y) => row.forEach((value, x) => {
      if (value) drawBlock(x, y, this.colors[y][x])
    }))

    if (this.status !== 'game-over') {
      let ghostY = this.piece.y
      while (!this.collides(this.piece.matrix, this.piece.x, ghostY + 1)) ghostY += 1
      this.piece.matrix.forEach((row, rowIndex) => row.forEach((value, colIndex) => {
        if (!value) return
        drawBlock(this.piece.x + colIndex, ghostY + rowIndex, this.piece.color, 0.18)
        drawBlock(this.piece.x + colIndex, this.piece.y + rowIndex, this.piece.color)
      }))
    }

    this.graphics.fillStyle(0x0f172a, 1)
    this.graphics.fillRoundedRect(565, 294, 190, 126, 8)
    this.nextPiece.matrix.forEach((row, rowIndex) => row.forEach((value, colIndex) => {
      if (!value) return
      this.graphics.fillStyle(this.nextPiece.color, 1)
      this.graphics.fillRoundedRect(605 + colIndex * 25, 320 + rowIndex * 25, 21, 21, 4)
    }))

    this.hud.setText(
      `SCORE ${String(this.score).padStart(5, '0')}\nLINES ${String(this.lines).padStart(3, '0')}\nLEVEL ${String(this.level).padStart(2, '0')}`,
    )
    if (this.status === 'game-over') {
      this.add.text(BOARD_X + 125, BOARD_Y + 250, 'GAME OVER', {
        color: '#fb7185',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: '28px',
        fontStyle: 'bold',
      }).setOrigin(0.5).setDepth(4)
    }
  }
}
