import { beforeEach, describe, expect, it, vi } from 'vitest'

const { gameConstructor } = vi.hoisted(() => ({
  gameConstructor: vi.fn(),
}))

vi.mock('phaser', () => {
  class Scene {
    readonly key: string

    constructor(key: string) {
      this.key = key
    }
  }

  class Game {
    constructor(config: unknown) {
      gameConstructor(config)
    }
  }

  return {
    default: {
      AUTO: 42,
      Game,
      Scene,
      Scale: {
        FIT: 'FIT',
        CENTER_BOTH: 'CENTER_BOTH',
      },
    },
  }
})

import { GAME_CATALOG, getGameById, isGameId } from '../catalog'
import { ARCADE_SCENE_BY_GAME_ID, calculateArcadeRenderScale, createArcadeGame } from '../phaser/createArcadeGame'
import { GAME_HEIGHT, GAME_WIDTH } from '../phaser/ArcadeScene'
import { GAME_IDS, type GameId, type SceneHooks } from '../types'

const EXPECTED_SCENES: Record<GameId, string> = {
  snake: 'SnakeScene',
  tetris: 'TetrisScene',
  breakout: 'BreakoutScene',
  merge2048: 'FruitMachineScene',
  orbital: 'OrbitalScene',
  hoarder: 'HoarderScene',
  lucky: 'LuckyScene',
}

describe('game catalog', () => {
  it('contains every GameId exactly once and resolves valid ids', () => {
    const catalogIds = GAME_CATALOG.map((game) => game.id)

    expect(catalogIds).toEqual([...GAME_IDS])
    expect(new Set(catalogIds).size).toBe(GAME_IDS.length)
    GAME_IDS.forEach((id) => {
      expect(getGameById(id)?.id).toBe(id)
      expect(isGameId(id)).toBe(true)
    })
    expect(getGameById('not-a-game')).toBeUndefined()
    expect(isGameId('not-a-game')).toBe(false)
  })

  it('maps every catalog entry to its intended scene class', () => {
    expect(Object.keys(ARCADE_SCENE_BY_GAME_ID)).toEqual([...GAME_IDS])
    GAME_IDS.forEach((id) => {
      expect(ARCADE_SCENE_BY_GAME_ID[id].name).toBe(EXPECTED_SCENES[id])
    })
  })
})

describe('createArcadeGame', () => {
  const hooks: SceneHooks = {
    onScore: vi.fn(),
    onStatus: vi.fn(),
  }

  beforeEach(() => {
    gameConstructor.mockClear()
  })

  it('uses a bounded high-DPI backing scale for the fruit machine only', () => {
    const parent = document.createElement('div')
    vi.spyOn(parent, 'getBoundingClientRect').mockReturnValue({
      width: 1030, height: 710, x: 0, y: 0, top: 0, left: 0, right: 1030, bottom: 710,
      toJSON: () => ({}),
    })

    expect(calculateArcadeRenderScale(parent, 'merge2048', 1.25)).toBe(1.5)
    expect(calculateArcadeRenderScale(parent, 'merge2048', 3)).toBe(2)
    expect(calculateArcadeRenderScale(parent, 'lucky', 3)).toBe(1)
  })

  it('constructs every game with a fixed, responsive Arcade configuration', () => {
    const parent = document.createElement('div')

    GAME_IDS.forEach((id) => createArcadeGame(parent, id, hooks))

    expect(gameConstructor).toHaveBeenCalledTimes(GAME_IDS.length)
    gameConstructor.mock.calls.forEach(([config], index) => {
      const gameId = GAME_IDS[index]
      expect(config).toMatchObject({
        type: 42,
        parent,
        width: GAME_WIDTH,
        height: GAME_HEIGHT,
        transparent: true,
        antialias: true,
        antialiasGL: true,
        pixelArt: false,
        roundPixels: false,
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
          mode: 'FIT',
          autoCenter: 'CENTER_BOTH',
          width: GAME_WIDTH,
          height: GAME_HEIGHT,
        },
        physics: {
          default: 'arcade',
          arcade: {
            gravity: { x: 0, y: 0 },
            debug: false,
          },
        },
      })
      expect((config as { scene: object }).scene).toBeInstanceOf(ARCADE_SCENE_BY_GAME_ID[gameId])
    })
  })
})
