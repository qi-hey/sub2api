import Phaser from 'phaser'
import { ArcadeScene } from '../ArcadeScene'
import { createCircleTexture, createRectTexture } from '../textureHelpers'
import type { GameControl, SceneHooks } from '../../types'

export const ORBITAL_ENEMY_POOL_SIZE = 32

export class OrbitalScene extends ArcadeScene {
  private ship!: Phaser.Physics.Arcade.Image
  private bullets!: Phaser.Physics.Arcade.Group
  private enemies!: Phaser.Physics.Arcade.Group
  private cursors?: Phaser.Types.Input.Keyboard.CursorKeys
  private hud!: Phaser.GameObjects.Text
  private lives = 3
  private wave = 1
  private nextShotAt = 0
  private spawnTimer?: Phaser.Time.TimerEvent

  constructor(hooks: SceneHooks) {
    super('orbital', hooks)
  }

  create(): void {
    this.lives = 3
    this.wave = 1
    this.nextShotAt = 0
    this.spawnTimer = undefined

    this.drawFrame('ORBITAL // DEFENSE GRID', 0x38bdf8)
    this.drawStars()
    createRectTexture(this, 'orbital-ship', 58, 24, 0x38bdf8, 8)
    createRectTexture(this, 'orbital-bullet', 6, 18, 0xf8fafc, 3)
    createCircleTexture(this, 'orbital-enemy', 34, 0xf472b6)

    this.physics.world.setBounds(62, 60, 690, 530)
    this.physics.world.setBoundsCollision(true, true, true, false)
    this.ship = this.physics.add.image(405, 545, 'orbital-ship').setCollideWorldBounds(true)
    if (this.ship.body instanceof Phaser.Physics.Arcade.Body) {
      this.ship.body.setAllowGravity(false)
    }
    this.bullets = this.physics.add.group({ maxSize: 28 })
    this.enemies = this.physics.add.group({ maxSize: ORBITAL_ENEMY_POOL_SIZE })
    this.physics.add.overlap(this.bullets, this.enemies, (bulletObject, enemyObject) => {
      const bullet = bulletObject as Phaser.Physics.Arcade.Image
      const enemy = enemyObject as Phaser.Physics.Arcade.Image
      if (!bullet.active || !enemy.active) return
      bullet.disableBody(true, true)
      enemy.disableBody(true, true)
      this.emitSound('orbital-hit')
      this.addScore(20)
      if (this.score > 0 && this.score % 300 === 0) this.wave += 1
      this.updateHud()
    })
    this.physics.add.overlap(this.ship, this.enemies, (_shipObject, enemyObject) => {
      const enemy = enemyObject as Phaser.Physics.Arcade.Image
      if (!enemy.active) return
      enemy.disableBody(true, true)
      this.loseLife()
    })

    this.hud = this.addHud('SCORE 0000\nLIVES 03\nWAVE  01', 770, 100)
    this.add.text(770, 250, 'LEFT / RIGHT\nSPACE: FIRE\nP: PAUSE\nR: RESTART', {
      color: '#64748b',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '13px',
      lineSpacing: 8,
    })
    this.cursors = this.input.keyboard?.createCursorKeys()
    this.input.keyboard?.on('keydown-SPACE', () => this.fire())
    this.input.keyboard?.on('keydown-P', () => this.handleControl('pause'))
    this.input.keyboard?.on('keydown-R', () => this.handleControl('restart'))
    this.input.on('pointermove', (pointer: Phaser.Input.Pointer) => {
      if (pointer.x >= 65 && pointer.x <= 750) this.moveShipTo(pointer.x)
    })
    this.input.on('pointerdown', () => this.fire())
    this.bindControls((control) => this.handleControl(control))
    this.spawnTimer = this.time.addEvent({ delay: 760, loop: true, callback: () => this.spawnEnemy() })
    this.begin()
    this.updateHud()
  }

  update(time: number): void {
    if (this.status !== 'playing') return
    if (this.cursors?.left.isDown) this.ship.setVelocityX(-360)
    else if (this.cursors?.right.isDown) this.ship.setVelocityX(360)
    else this.ship.setVelocityX(0)

    if (this.cursors?.space.isDown && time >= this.nextShotAt) this.fire()
    this.bullets.children.each((child) => {
      const bullet = child as Phaser.Physics.Arcade.Image
      if (bullet.active && bullet.y < 58) bullet.disableBody(true, true)
      return true
    })
    this.enemies.children.each((child) => {
      const enemy = child as Phaser.Physics.Arcade.Image
      if (enemy.active && enemy.y > 590) {
        enemy.disableBody(true, true)
        this.loseLife()
      }
      return true
    })
  }

  private handleControl(control: GameControl): void {
    if (this.handleCommonControl(control)) return
    if (this.status !== 'playing') return
    if (control === 'left') this.moveShipTo(this.ship.x - 65)
    if (control === 'right') this.moveShipTo(this.ship.x + 65)
    if (control === 'action' || control === 'up') this.fire()
  }

  private fire(): void {
    if (this.status !== 'playing' || this.time.now < this.nextShotAt) return
    const bullet = this.bullets.get(this.ship.x, this.ship.y - 24, 'orbital-bullet') as Phaser.Physics.Arcade.Image | null
    if (!bullet) return
    bullet.enableBody(true, this.ship.x, this.ship.y - 24, true, true)
    if (bullet.body instanceof Phaser.Physics.Arcade.Body) {
      bullet.body.setAllowGravity(false)
    }
    bullet.setVelocityY(-520)
    this.nextShotAt = this.time.now + 180
    this.emitSound('orbital-shoot')
  }

  private spawnEnemy(): void {
    if (this.status !== 'playing') return
    const x = Phaser.Math.Between(90, 720)
    const enemy = this.enemies.get(x, 82, 'orbital-enemy') as Phaser.Physics.Arcade.Image | null
    if (!enemy) return

    enemy.enableBody(true, x, 82, true, true)
    if (enemy.body instanceof Phaser.Physics.Arcade.Body) {
      enemy.body.setAllowGravity(false)
    }
    enemy.setVelocity(Phaser.Math.Between(-55, 55), Phaser.Math.Between(100, 145) + this.wave * 8)
    enemy.setBounce(1, 0)
    enemy.setCollideWorldBounds(true)
  }

  private moveShipTo(x: number): void {
    this.ship.setX(Phaser.Math.Clamp(x, 94, 720))
    this.ship.body?.updateFromGameObject()
  }

  private loseLife(): void {
    if (this.status !== 'playing') return
    this.lives -= 1
    this.emitSound('orbital-life')
    this.updateHud()
    this.cameras.main.shake(120, 0.006)
    if (this.lives > 0) return
    this.spawnTimer?.remove(false)
    this.ship.setVelocity(0, 0)
    this.finish()
    this.add.text(405, 318, 'DEFENSE OFFLINE', {
      color: '#fb7185',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '30px',
      fontStyle: 'bold',
    }).setOrigin(0.5)
  }

  private updateHud(): void {
    this.hud?.setText(
      `SCORE ${String(this.score).padStart(4, '0')}\nLIVES ${String(this.lives).padStart(2, '0')}\nWAVE  ${String(this.wave).padStart(2, '0')}`,
    )
  }

  private drawStars(): void {
    const graphics = this.add.graphics()
    for (let index = 0; index < 90; index += 1) {
      const alpha = Phaser.Math.FloatBetween(0.16, 0.68)
      graphics.fillStyle(index % 9 === 0 ? 0x38bdf8 : 0xffffff, alpha)
      graphics.fillCircle(Phaser.Math.Between(66, 746), Phaser.Math.Between(72, 580), Phaser.Math.FloatBetween(0.5, 1.8))
    }
  }
}
