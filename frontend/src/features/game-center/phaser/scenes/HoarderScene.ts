import Phaser from 'phaser'
import { ArcadeScene } from '../ArcadeScene'
import type { GameControl, SceneHooks } from '../../types'

interface Hole {
  x: number
  y: number
  target: Phaser.GameObjects.Container
}

export class HoarderScene extends ArcadeScene {
  private holes: Hole[] = []
  private activeHole?: Hole
  private hud!: Phaser.GameObjects.Text
  private timeLeft = 45
  private spawnTimer?: Phaser.Time.TimerEvent
  private countdownTimer?: Phaser.Time.TimerEvent
  private hideTimer?: Phaser.Time.TimerEvent

  constructor(hooks: SceneHooks) {
    super('hoarder', hooks)
  }

  create(): void {
    this.holes = []
    this.activeHole = undefined
    this.spawnTimer = undefined
    this.countdownTimer = undefined
    this.hideTimer = undefined
    this.timeLeft = 45

    this.drawFrame('HOARDER // QUICK STRIKE', 0xa3e635)
    const field = this.add.graphics()
    field.fillStyle(0x111827, 1)
    field.fillRoundedRect(92, 84, 610, 474, 12)
    field.lineStyle(1, 0xa3e635, 0.12)
    field.strokeRoundedRect(92, 84, 610, 474, 12)

    for (let row = 0; row < 3; row += 1) {
      for (let column = 0; column < 3; column += 1) {
        const x = 190 + column * 205
        const y = 172 + row * 145
        field.fillStyle(0x030712, 1)
        field.fillEllipse(x, y + 30, 124, 42)
        field.lineStyle(3, 0x65a30d, 0.55)
        field.strokeEllipse(x, y + 30, 124, 42)

        const body = this.add.circle(0, 0, 38, 0xd97706)
        const face = this.add.circle(0, -7, 31, 0xfbbf24)
        const leftEar = this.add.circle(-25, -28, 10, 0xfb923c)
        const rightEar = this.add.circle(25, -28, 10, 0xfb923c)
        const leftEye = this.add.circle(-10, -10, 4, 0x111827)
        const rightEye = this.add.circle(10, -10, 4, 0x111827)
        const nose = this.add.circle(0, 2, 5, 0xf43f5e)
        const target = this.add.container(x, y + 12, [leftEar, rightEar, body, face, leftEye, rightEye, nose])
        target.setSize(84, 84)
        target.setInteractive({ cursor: 'pointer' })
        target.setVisible(false)
        const hole: Hole = { x, y, target }
        target.on('pointerdown', () => this.hitTarget(hole))
        this.holes.push(hole)
      }
    }

    this.hud = this.addHud('SCORE 000\nTIME  45', 744, 122)
    this.add.text(744, 226, 'CLICK THE TARGET\nSPACE: STRIKE\nP: PAUSE\nR: RESTART', {
      color: '#64748b',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '13px',
      lineSpacing: 8,
    })
    this.begin()
    this.bindControls((control) => this.handleControl(control))
    this.input.keyboard?.on('keydown-SPACE', () => this.handleControl('action'))
    this.input.keyboard?.on('keydown-P', () => this.handleControl('pause'))
    this.input.keyboard?.on('keydown-R', () => this.handleControl('restart'))
    this.spawnTimer = this.time.addEvent({ delay: 680, loop: true, callback: () => this.showTarget() })
    this.countdownTimer = this.time.addEvent({ delay: 1000, loop: true, callback: () => this.countdown() })
    this.showTarget()
    this.updateHud()
  }

  private handleControl(control: GameControl): void {
    if (this.handleCommonControl(control)) return
    if (control === 'action' && this.activeHole) this.hitTarget(this.activeHole)
  }

  private showTarget(): void {
    if (this.status !== 'playing') return
    this.activeHole?.target.setVisible(false)
    this.hideTimer?.remove(false)
    const choices = this.holes.filter((hole) => hole !== this.activeHole)
    const next = Phaser.Utils.Array.GetRandom(choices) ?? this.holes[0]
    this.activeHole = next
    next.target.setVisible(true)
    next.target.setScale(0.25)
    this.tweens.add({ targets: next.target, scale: 1, duration: 130, ease: 'Back.Out' })
    this.hideTimer = this.time.delayedCall(Math.max(330, 610 - Math.floor(this.score / 5) * 12), () => {
      next.target.setVisible(false)
      if (this.activeHole === next) this.activeHole = undefined
    })
  }

  private hitTarget(hole: Hole): void {
    if (this.status !== 'playing' || this.activeHole !== hole || !hole.target.visible) return
    hole.target.setVisible(false)
    this.activeHole = undefined
    this.hideTimer?.remove(false)
    this.addScore(1)
    this.emitSound('hoarder-hit')
    this.updateHud()
    this.time.delayedCall(90, () => this.showTarget())
  }

  private countdown(): void {
    if (this.status !== 'playing') return
    this.timeLeft -= 1
    this.updateHud()
    if (this.timeLeft > 0) return
    this.spawnTimer?.remove(false)
    this.countdownTimer?.remove(false)
    this.hideTimer?.remove(false)
    this.activeHole?.target.setVisible(false)
    this.emitSound('hoarder-countdown-end')
    this.finish('complete')
    this.add.text(397, 320, 'TIME UP', {
      color: '#fbbf24',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '34px',
      fontStyle: 'bold',
    }).setOrigin(0.5)
  }

  private updateHud(): void {
    this.hud?.setText(`SCORE ${String(this.score).padStart(3, '0')}\nTIME  ${String(this.timeLeft).padStart(2, '0')}`)
  }
}
