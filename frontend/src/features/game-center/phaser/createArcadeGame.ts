import Phaser from 'phaser'
import type { GameId, SceneHooks } from '../types'
import { ArcadeScene, GAME_HEIGHT, GAME_WIDTH } from './ArcadeScene'
import { BreakoutScene } from './scenes/BreakoutScene'
import { HoarderScene } from './scenes/HoarderScene'
import { LuckyScene } from './scenes/LuckyScene'
import { FruitMachineScene } from './scenes/FruitMachineScene'
import { OrbitalScene } from './scenes/OrbitalScene'
import { SnakeScene } from './scenes/SnakeScene'
import { TetrisScene } from './scenes/TetrisScene'

export type ArcadeSceneConstructor = new (hooks: SceneHooks) => ArcadeScene

const FRUIT_RENDER_SCALE_STEP = 0.25
const FRUIT_MAX_RENDER_SCALE = 2

export const ARCADE_SCENE_BY_GAME_ID: Record<GameId, ArcadeSceneConstructor> = {
  snake: SnakeScene,
  tetris: TetrisScene,
  breakout: BreakoutScene,
  merge2048: FruitMachineScene,
  orbital: OrbitalScene,
  hoarder: HoarderScene,
  lucky: LuckyScene,
}

export function calculateArcadeRenderScale(
  parent: HTMLElement | string,
  gameId: GameId,
  devicePixelRatio = typeof window === 'undefined' ? 1 : window.devicePixelRatio,
): number {
  if (gameId !== 'merge2048' || typeof parent === 'string') return 1

  const bounds = parent.getBoundingClientRect()
  const cssWidth = bounds.width || parent.clientWidth || GAME_WIDTH
  const cssHeight = bounds.height || parent.clientHeight || GAME_HEIGHT
  const cssScale = Math.max(cssWidth / GAME_WIDTH, cssHeight / GAME_HEIGHT)
  const pixelRatio = Math.max(1, Math.min(3, devicePixelRatio || 1))
  const requested = Math.max(1, cssScale * pixelRatio)
  const stepped = Math.ceil(requested / FRUIT_RENDER_SCALE_STEP) * FRUIT_RENDER_SCALE_STEP
  return Math.min(FRUIT_MAX_RENDER_SCALE, stepped)
}

export function createArcadeGame(
  parent: HTMLElement | string,
  gameId: GameId,
  hooks: SceneHooks,
): Phaser.Game {
  const Scene = ARCADE_SCENE_BY_GAME_ID[gameId]
  const renderScale = calculateArcadeRenderScale(parent, gameId)
  const renderWidth = Math.round(GAME_WIDTH * renderScale)
  const renderHeight = Math.round(GAME_HEIGHT * renderScale)
  const config: Phaser.Types.Core.GameConfig = {
    type: Phaser.AUTO,
    parent,
    width: renderWidth,
    height: renderHeight,
    transparent: true,
    antialias: true,
    antialiasGL: true,
    pixelArt: false,
    roundPixels: false,
    autoFocus: true,
    disableContextMenu: true,
    fps: {
      target: 60,
      limit: 60,
      forceSetTimeOut: false,
      smoothStep: true,
    },
    render: {
      powerPreference: 'high-performance',
    },
    scale: {
      mode: Phaser.Scale.FIT,
      autoCenter: Phaser.Scale.CENTER_BOTH,
      width: renderWidth,
      height: renderHeight,
    },
    physics: {
      default: 'arcade',
      arcade: {
        gravity: { x: 0, y: 0 },
        debug: false,
      },
    },
    scene: new Scene({ ...hooks, renderScale }),
  }

  return new Phaser.Game(config)
}
