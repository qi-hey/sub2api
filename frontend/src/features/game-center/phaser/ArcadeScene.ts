import Phaser from 'phaser'
import { SOUND_EVENT, type ArcadeSound } from '../audio'
import type { GameControl, GameStatus, SceneHooks } from '../types'

export const GAME_WIDTH = 900
export const GAME_HEIGHT = 620
export const CONTROL_EVENT = 'sub2api-game-control'

export abstract class ArcadeScene extends Phaser.Scene {
  protected score = 0
  protected status: GameStatus = 'ready'
  protected readonly hooks: SceneHooks
  private controlHandler?: (control: GameControl) => void

  protected constructor(key: string, hooks: SceneHooks) {
    super(key)
    this.hooks = hooks
  }

  protected begin(): void {
    this.score = 0
    this.setStatus('playing')
    this.setScore(0)
  }

  protected setScore(score: number): void {
    this.score = Math.max(0, Math.floor(score))
    this.hooks.onScore(this.score)
  }

  protected addScore(points: number): void {
    this.setScore(this.score + points)
  }

  protected setStatus(status: GameStatus): void {
    this.status = status
    this.hooks.onStatus(status)
  }

  protected bindControls(handler: (control: GameControl) => void): void {
    this.controlHandler = handler
    this.game.events.on(CONTROL_EVENT, handler)
    this.events.once(Phaser.Scenes.Events.SHUTDOWN, () => {
      if (this.controlHandler) {
        this.game.events.off(CONTROL_EVENT, this.controlHandler)
      }
    })
  }

  protected handleCommonControl(control: GameControl): boolean {
    if (control === 'restart') {
      if (this.canRestart()) this.scene.restart()
      return true
    }
    if (control !== 'pause' || this.status === 'game-over' || this.status === 'complete') {
      return false
    }

    if (this.status === 'paused') {
      this.scene.resume()
      this.setStatus('playing')
    } else {
      this.setStatus('paused')
      this.scene.pause()
    }
    return true
  }

  protected canRestart(): boolean {
    return true
  }

  protected drawFrame(title: string, accent: number): void {
    this.cameras.main.setBackgroundColor('#05070d')
    const frame = this.add.graphics()
    frame.lineStyle(2, accent, 0.65)
    frame.strokeRoundedRect(18, 18, GAME_WIDTH - 36, GAME_HEIGHT - 36, 12)
    frame.lineStyle(1, accent, 0.18)
    frame.strokeRoundedRect(26, 26, GAME_WIDTH - 52, GAME_HEIGHT - 52, 10)
    this.add.text(42, 34, title, {
      color: Phaser.Display.Color.IntegerToColor(accent).rgba,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '18px',
      fontStyle: 'bold',
    })
  }

  protected addHud(label: string, x = 710, y = 72): Phaser.GameObjects.Text {
    return this.add.text(x, y, label, {
      color: '#e2e8f0',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '18px',
      lineSpacing: 9,
    })
  }

  protected emitSound(sound: ArcadeSound): void {
    this.game.events.emit(SOUND_EVENT, sound)
  }

  protected finish(status: Extract<GameStatus, 'game-over' | 'complete'> = 'game-over'): void {
    if (this.status === 'game-over' || this.status === 'complete') return
    this.emitSound(status)
    this.setStatus(status)
  }
}
