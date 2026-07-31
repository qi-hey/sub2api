import { describe, expect, it, vi } from 'vitest'

vi.mock('phaser', () => {
  class Scene {
    readonly key: string

    constructor(key: string) {
      this.key = key
    }
  }

  class Body {
    velocity = { x: 0, y: 0 }
    setAllowGravity = vi.fn()
    updateFromGameObject = vi.fn()
  }

  return {
    default: {
      Scene,
      Physics: {
        Arcade: {
          Body,
        },
      },
      Math: {
        Between: vi.fn((min: number) => min),
        FloatBetween: vi.fn((min: number) => min),
        Clamp: vi.fn((value: number, min: number, max: number) => Math.min(max, Math.max(min, value))),
      },
      Display: {
        Color: {
          IntegerToColor: vi.fn(() => ({ rgba: '#38bdf8' })),
        },
      },
      Scenes: {
        Events: {
          SHUTDOWN: 'shutdown',
        },
      },
    },
  }
})

import Phaser from 'phaser'
import { ORBITAL_ENEMY_POOL_SIZE, OrbitalScene } from '../phaser/scenes/OrbitalScene'
import type { SceneHooks } from '../types'

interface MockBody {
  velocity: { x: number; y: number }
  setAllowGravity: ReturnType<typeof vi.fn>
  updateFromGameObject: ReturnType<typeof vi.fn>
}

interface MockEnemy {
  active: boolean
  body: MockBody
  enableBody: ReturnType<typeof vi.fn>
  disableBody: ReturnType<typeof vi.fn>
  setVelocity: ReturnType<typeof vi.fn>
  setBounce: ReturnType<typeof vi.fn>
  setCollideWorldBounds: ReturnType<typeof vi.fn>
}

interface MockGroup {
  maxSize: number
  entries: MockEnemy[]
  get: ReturnType<typeof vi.fn>
  children: {
    each: ReturnType<typeof vi.fn>
  }
}

function createEnemy(): MockEnemy {
  const enemy = {
    active: false,
    body: createBody(),
    enableBody: vi.fn(),
    disableBody: vi.fn(),
    setVelocity: vi.fn(),
    setBounce: vi.fn(),
    setCollideWorldBounds: vi.fn(),
  } as MockEnemy

  enemy.enableBody.mockImplementation(() => {
    enemy.active = true
    return enemy
  })
  enemy.disableBody.mockImplementation(() => {
    enemy.active = false
    return enemy
  })
  enemy.setVelocity.mockReturnValue(enemy)
  enemy.setBounce.mockReturnValue(enemy)
  enemy.setCollideWorldBounds.mockReturnValue(enemy)
  return enemy
}

function createBody(): MockBody {
  const Body = Phaser.Physics.Arcade.Body as unknown as new () => MockBody
  return new Body()
}

function createGroup(maxSize = -1): MockGroup {
  const entries: MockEnemy[] = []
  const group = {
    maxSize,
    entries,
    get: vi.fn(),
    children: {
      each: vi.fn(),
    },
  } as MockGroup

  group.get.mockImplementation(() => {
    const available = entries.find((entry) => !entry.active)
    if (available) return available
    if (maxSize >= 0 && entries.length >= maxSize) return null

    const enemy = createEnemy()
    entries.push(enemy)
    return enemy
  })
  group.children.each.mockImplementation((callback: (child: MockEnemy) => boolean) => {
    entries.forEach(callback)
  })
  return group
}

function attachRuntime(scene: OrbitalScene): MockGroup[] {
  const groups: MockGroup[] = []
  const group = vi.fn((config: { maxSize?: number } = {}) => {
    const created = createGroup(config.maxSize)
    groups.push(created)
    return created
  })
  const graphics = {
    fillStyle: vi.fn(),
    fillCircle: vi.fn(),
    fillRoundedRect: vi.fn(),
    lineStyle: vi.fn(),
    strokeRoundedRect: vi.fn(),
    generateTexture: vi.fn(),
    destroy: vi.fn(),
  }
  const text = {
    setOrigin: vi.fn(),
    setText: vi.fn(),
  }
  text.setOrigin.mockReturnValue(text)
  text.setText.mockReturnValue(text)
  const ship = {
    active: true,
    x: 405,
    y: 545,
    body: createBody(),
    setCollideWorldBounds: vi.fn(),
    setVelocity: vi.fn(),
    setVelocityX: vi.fn(),
    setX: vi.fn(),
  }
  ship.setCollideWorldBounds.mockReturnValue(ship)
  ship.setVelocity.mockReturnValue(ship)
  ship.setVelocityX.mockReturnValue(ship)
  ship.setX.mockReturnValue(ship)

  Object.assign(scene, {
    add: {
      graphics: vi.fn(() => graphics),
      text: vi.fn(() => text),
    },
    cameras: {
      main: {
        setBackgroundColor: vi.fn(),
        shake: vi.fn(),
      },
    },
    textures: {
      exists: vi.fn(() => false),
    },
    physics: {
      world: {
        setBounds: vi.fn(),
        setBoundsCollision: vi.fn(),
      },
      add: {
        image: vi.fn(() => ship),
        group,
        overlap: vi.fn(),
      },
    },
    input: {
      keyboard: {
        createCursorKeys: vi.fn(() => ({})),
        on: vi.fn(),
      },
      on: vi.fn(),
    },
    game: {
      events: {
        on: vi.fn(),
        off: vi.fn(),
        emit: vi.fn(),
      },
    },
    events: {
      once: vi.fn(),
    },
    time: {
      now: 0,
      addEvent: vi.fn(() => ({ remove: vi.fn() })),
    },
  })

  return groups
}

describe('OrbitalScene enemy pool', () => {
  it('caps enemy objects and reuses inactive entries without throwing when full', () => {
    const hooks: SceneHooks = {
      onScore: vi.fn(),
      onStatus: vi.fn(),
    }
    const scene = new OrbitalScene(hooks)
    const groups = attachRuntime(scene)
    scene.create()

    const enemyGroup = groups[1]
    const spawnEnemy = (scene as unknown as { spawnEnemy: () => void }).spawnEnemy.bind(scene)

    expect(enemyGroup.maxSize).toBe(ORBITAL_ENEMY_POOL_SIZE)
    expect(() => {
      for (let index = 0; index < ORBITAL_ENEMY_POOL_SIZE * 3; index += 1) spawnEnemy()
    }).not.toThrow()
    expect(enemyGroup.entries).toHaveLength(ORBITAL_ENEMY_POOL_SIZE)

    const recycledEnemy = enemyGroup.entries[0]
    recycledEnemy.disableBody(true, true)
    spawnEnemy()

    expect(enemyGroup.entries).toHaveLength(ORBITAL_ENEMY_POOL_SIZE)
    expect(recycledEnemy.enableBody).toHaveBeenCalledTimes(2)
  })
})
