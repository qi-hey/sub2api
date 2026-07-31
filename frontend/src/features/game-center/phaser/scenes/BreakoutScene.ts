import Phaser from 'phaser'
import { ArcadeScene } from '../ArcadeScene'
import { createCircleTexture, createRectTexture } from '../textureHelpers'
import type { GameControl, SceneHooks } from '../../types'

export class BreakoutScene extends ArcadeScene {
  private paddle!: Phaser.Physics.Arcade.Image
  private ball!: Phaser.Physics.Arcade.Image
  private bricks!: Phaser.Physics.Arcade.StaticGroup
  private cursors?: Phaser.Types.Input.Keyboard.CursorKeys
  private hud!: Phaser.GameObjects.Text
  private launched = false
  private lives = 3
  private brickCount = 0

  constructor(hooks: SceneHooks) {
    super('breakout', hooks)
  }

  create(): void {
    this.launched = false
    this.lives = 3
    this.brickCount = 0

    this.drawFrame('BREAKOUT // NEON CIRCUIT', 0xf472b6)
    createRectTexture(this, 'breakout-paddle', 118, 18, 0x22d3ee, 8)
    createCircleTexture(this, 'breakout-ball', 18, 0xf8fafc)
    const colors = [0xf43f5e, 0xf97316, 0xfbbf24, 0x4ade80, 0x38bdf8]
    colors.forEach((color, index) => createRectTexture(this, `breakout-brick-${index}`, 54, 22, color, 4))

    this.physics.world.setBounds(78, 70, 650, 520)
    this.physics.world.setBoundsCollision(true, true, true, false)
    this.paddle = this.physics.add.image(400, 555, 'breakout-paddle').setImmovable(true)
    if (this.paddle.body instanceof Phaser.Physics.Arcade.Body) {
      this.paddle.body.setAllowGravity(false)
    }
    this.ball = this.physics.add.image(400, 530, 'breakout-ball')
      .setBounce(1, 1)
      .setCollideWorldBounds(true, 1, 1, true)
    if (this.ball.body instanceof Phaser.Physics.Arcade.Body) {
      this.ball.body.setAllowGravity(false)
    }
    this.ball.setData('onPaddle', true)

    this.bricks = this.physics.add.staticGroup()
    colors.forEach((_, row) => {
      for (let column = 0; column < 10; column += 1) {
        const brick = this.bricks.create(116 + column * 62, 118 + row * 34, `breakout-brick-${row}`) as Phaser.Physics.Arcade.Image
        brick.refreshBody()
        this.brickCount += 1
      }
    })

    this.physics.add.collider(this.ball, this.paddle, (_ball, paddleObject) => {
      const paddle = paddleObject as Phaser.Physics.Arcade.Image
      const offset = Phaser.Math.Clamp((this.ball.x - paddle.x) / 60, -1, 1)
      const velocityY = this.ball.body instanceof Phaser.Physics.Arcade.Body ? this.ball.body.velocity.y : 0
      this.ball.setVelocity(330 * offset, -Math.max(280, Math.abs(velocityY)))
    })
    this.physics.add.collider(this.ball, this.bricks, (_ball, brickObject) => {
      const brick = brickObject as Phaser.Physics.Arcade.Image
      if (!brick.active) return
      brick.disableBody(true, true)
      this.brickCount -= 1
      this.emitSound('breakout-brick')
      this.addScore(25)
      this.updateHud()
      if (this.brickCount === 0) {
        this.ball.setVelocity(0, 0)
        this.finish('complete')
        this.add.text(402, 324, 'BOARD CLEARED', {
          color: '#4ade80',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
          fontSize: '30px',
          fontStyle: 'bold',
        }).setOrigin(0.5)
      }
    })

    this.hud = this.addHud('SCORE 0000\nLIVES 03', 752, 104)
    this.add.text(752, 214, 'LEFT / RIGHT\nSPACE: LAUNCH\nP: PAUSE\nR: RESTART', {
      color: '#64748b',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '13px',
      lineSpacing: 8,
    })
    this.cursors = this.input.keyboard?.createCursorKeys()
    this.input.keyboard?.on('keydown-SPACE', () => this.launchBall())
    this.input.keyboard?.on('keydown-P', () => this.handleControl('pause'))
    this.input.keyboard?.on('keydown-R', () => this.handleControl('restart'))
    this.input.on('pointermove', (pointer: Phaser.Input.Pointer) => {
      if (pointer.x < 75 || pointer.x > 730) return
      this.movePaddleTo(pointer.x)
    })
    this.input.on('pointerdown', () => this.launchBall())
    this.bindControls((control) => this.handleControl(control))
    this.begin()
    this.updateHud()
  }

  update(): void {
    if (this.status !== 'playing') return
    if (this.cursors?.left.isDown) this.movePaddleTo(this.paddle.x - 9)
    if (this.cursors?.right.isDown) this.movePaddleTo(this.paddle.x + 9)
    if (!this.launched) {
      this.resetBallToPaddle()
    } else if (this.ball.y > this.physics.world.bounds.bottom + this.ball.displayHeight) {
      this.loseLife()
    }
  }

  private handleControl(control: GameControl): void {
    if (this.handleCommonControl(control)) return
    if (this.status !== 'playing') return
    if (control === 'left') this.movePaddleTo(this.paddle.x - 70)
    if (control === 'right') this.movePaddleTo(this.paddle.x + 70)
    if (control === 'action' || control === 'up') this.launchBall()
  }

  private movePaddleTo(x: number): void {
    this.paddle.setX(Phaser.Math.Clamp(x, 136, 668))
    this.paddle.body?.updateFromGameObject()
    if (!this.launched) this.resetBallToPaddle()
  }

  private launchBall(): void {
    if (this.status !== 'playing' || this.launched) return
    this.launched = true
    this.ball.setVelocity(Phaser.Math.Between(-180, 180), -340)
    this.emitSound('breakout-launch')
  }

  private loseLife(): void {
    this.lives -= 1
    this.emitSound('breakout-life')
    this.updateHud()
    if (this.lives <= 0) {
      this.ball.setVelocity(0, 0)
      this.finish()
      this.add.text(402, 324, 'GAME OVER', {
        color: '#fb7185',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: '30px',
        fontStyle: 'bold',
      }).setOrigin(0.5)
      return
    }
    this.launched = false
    this.ball.setVelocity(0, 0)
    this.resetBallToPaddle()
  }

  private resetBallToPaddle(): void {
    this.ball.setPosition(this.paddle.x, this.paddle.y - 25)
    this.ball.body?.updateFromGameObject()
  }

  private updateHud(): void {
    this.hud?.setText(`SCORE ${String(this.score).padStart(4, '0')}\nLIVES ${String(this.lives).padStart(2, '0')}`)
  }
}
