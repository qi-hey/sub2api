import { describe, expect, it, vi } from 'vitest'

vi.mock('phaser', () => {
  class Scene {
    scene = { restart: vi.fn(), pause: vi.fn(), resume: vi.fn() }
  }
  return {
    default: {
      Scene,
      Scenes: { Events: { SHUTDOWN: 'shutdown' } },
    },
  }
})

import {
  createFruitTrainRuns,
  createFruitRunDelays,
  fruitTimelineWaitMs,
  FRUIT_ANIMATION_TIMING,
  FRUIT_BET_HOLD_TIMING,
  FRUIT_REFERENCE_CELLS,
  FRUIT_TRAIN_TIMING,
  FruitMachineScene,
} from '../phaser/scenes/FruitMachineScene'
import type { SceneHooks } from '../types'

describe('FruitMachineScene', () => {
  it('keeps the physical 24-cell board order and visible multipliers', () => {
    expect(FRUIT_REFERENCE_CELLS).toHaveLength(24)
    expect(FRUIT_REFERENCE_CELLS.map((cell) => (
      cell.special ? 'special' : `${cell.symbol}:${cell.size}:${cell.multiplier}`
    ))).toEqual([
      'orange:big:20',
      'bell:big:20',
      'bar:big:60',
      'bar:big:120',
      'bar:big:80',
      'apple:big:6',
      'mango:big:20',
      'watermelon:big:40',
      'watermelon:small:3',
      'special',
      'apple:big:6',
      'orange:small:3',
      'orange:big:20',
      'bell:big:20',
      '77:small:3',
      '77:big:40',
      'apple:big:6',
      'mango:small:3',
      'mango:big:20',
      'star:big:40',
      'star:small:3',
      'special',
      'apple:big:6',
      'bell:small:3',
    ])
    expect(FRUIT_REFERENCE_CELLS.flatMap((cell) => cell.multiplier ?? []))
      .toEqual(expect.arrayContaining([3, 6, 20, 40, 60, 80, 120]))
    expect(FRUIT_REFERENCE_CELLS.flatMap((cell, index) => cell.special ? [index] : []))
      .toEqual([9, 21])
  })

  it('matches the reference center and outer-run audio lengths while easing into the stop', () => {
    const outerSteps = FRUIT_ANIMATION_TIMING.outerLaps * FRUIT_REFERENCE_CELLS.length
    const outerDelays = createFruitRunDelays(outerSteps, FRUIT_ANIMATION_TIMING.outerRunMs)
    const landingMs = FRUIT_ANIMATION_TIMING.landingFlashMs * 4

    expect(FRUIT_ANIMATION_TIMING.centerPreludeSteps * FRUIT_ANIMATION_TIMING.centerPreludeStepMs)
      .toBe(7280)
    expect(outerDelays.reduce((sum, delay) => sum + delay, 0)).toBe(5560)
    expect(outerDelays[outerDelays.length - 1]).toBeGreaterThan(outerDelays[0])
    expect(FRUIT_ANIMATION_TIMING.outerRunMs + landingMs).toBe(5900)
    expect(FRUIT_ANIMATION_TIMING.centerRunMs + landingMs).toBe(7300)
  })

  it('uses absolute soundtrack deadlines instead of accumulating timer drift', () => {
    expect(fruitTimelineWaitMs(1000, 112, 1030)).toBe(82)
    expect(fruitTimelineWaitMs(1000, 224, 1240)).toBe(0)
  })

  it('builds a wrapped five-light train route without jumping between reward stops', () => {
    const [first, second] = createFruitTrainRuns(22, [2, 1])

    expect(first.positions).toHaveLength(FRUIT_TRAIN_TIMING.firstRunLaps * 24 + 4)
    expect(first.positions.slice(0, 4)).toEqual([23, 0, 1, 2])
    expect(first.positions.at(-1)).toBe(2)
    expect(first.totalMs).toBe(FRUIT_TRAIN_TIMING.firstRunMs)

    expect(second.positions).toHaveLength(FRUIT_TRAIN_TIMING.nextRunLaps * 24 + 23)
    expect(second.positions[0]).toBe(3)
    expect(second.positions.at(-1)).toBe(1)
    expect(second.totalMs).toBe(FRUIT_TRAIN_TIMING.nextRunMs)
    expect(FRUIT_TRAIN_TIMING.trailLength).toBe(5)
  })

  it('adds immediately, repeats after 300 ms, and stops repeating on release', () => {
    let startCallback: (() => void) | undefined
    let repeatCallback: (() => void) | undefined
    const removeStart = vi.fn()
    const removeRepeat = vi.fn()
    const scene = new FruitMachineScene({ onScore: vi.fn(), onStatus: vi.fn() })
    const state = scene as unknown as {
      status: 'playing'
      bets: number[]
      betDoors: unknown[]
      game: { events: { emit: ReturnType<typeof vi.fn> } }
      time: {
        delayedCall: (delay: number, callback: () => void) => { remove: typeof removeStart }
        addEvent: (config: { delay: number; callback: () => void }) => { remove: typeof removeRepeat }
      }
      startBetHold: (index: number, pointer: { id: number }) => void
      stopBetHold: (pointer: { id: number }) => void
    }
    state.status = 'playing'
    state.bets = Array<number>(8).fill(0)
    state.betDoors = []
    state.game = { events: { emit: vi.fn() } }
    state.time = {
      delayedCall: (delay, callback) => {
        expect(delay).toBe(FRUIT_BET_HOLD_TIMING.initialDelayMs)
        startCallback = callback
        return { remove: removeStart }
      },
      addEvent: ({ delay, callback }) => {
        expect(delay).toBe(FRUIT_BET_HOLD_TIMING.repeatMs)
        repeatCallback = callback
        return { remove: removeRepeat }
      },
    }

    state.startBetHold(0, { id: 7 })
    expect(state.bets[0]).toBe(1)
    startCallback?.()
    repeatCallback?.()
    expect(state.bets[0]).toBe(2)

    state.stopBetHold({ id: 7 })
    repeatCallback?.()
    expect(state.bets[0]).toBe(2)
    expect(removeRepeat).toHaveBeenCalled()
  })

  it('stops hold-to-bet at the existing 100-credit per-door cap', () => {
    const delayedCall = vi.fn()
    const scene = new FruitMachineScene({ onScore: vi.fn(), onStatus: vi.fn() })
    const state = scene as unknown as {
      status: 'playing'
      bets: number[]
      betDoors: unknown[]
      game: { events: { emit: ReturnType<typeof vi.fn> } }
      time: { delayedCall: typeof delayedCall }
      startBetHold: (index: number, pointer: { id: number }) => void
    }
    state.status = 'playing'
    state.bets = [99, 0, 0, 0, 0, 0, 0, 0]
    state.betDoors = []
    state.game = { events: { emit: vi.fn() } }
    state.time = { delayedCall }

    state.startBetHold(0, { id: 3 })

    expect(state.bets[0]).toBe(100)
    expect(delayedCall).not.toHaveBeenCalled()
  })

  it('does not submit a paid spin while the scene is paused', async () => {
    const requestFruitSpin = vi.fn()
    const hooks: SceneHooks = {
      onScore: vi.fn(),
      onStatus: vi.fn(),
      getFruitState: () => ({ credits: '100', loyaltyEnabled: true, loyaltyAvailable: true }),
      requestFruitSpin,
    }
    const scene = new FruitMachineScene(hooks)
    const state = scene as unknown as {
      status: 'paused'
      bets: number[]
      startSpin: () => Promise<void>
    }
    state.status = 'paused'
    state.bets = [1, 0, 0, 0, 0, 0, 0, 0]

    await state.startSpin()

    expect(requestFruitSpin).not.toHaveBeenCalled()
  })
})
