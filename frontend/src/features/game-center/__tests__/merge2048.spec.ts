import { describe, expect, it } from 'vitest'
import { canMove2048, move2048, type Grid2048 } from '../phaser/merge2048'

describe('move2048', () => {
  it('compacts and merges each pair only once', () => {
    const grid: Grid2048 = [
      [2, 2, 2, 2],
      [4, 0, 4, 4],
      [0, 0, 0, 0],
      [2, 4, 8, 16],
    ]

    expect(move2048(grid, 'left')).toEqual({
      grid: [
        [4, 4, 0, 0],
        [8, 4, 0, 0],
        [0, 0, 0, 0],
        [2, 4, 8, 16],
      ],
      score: 16,
      moved: true,
    })
  })

  it('moves right without mutating the input grid', () => {
    const grid: Grid2048 = [
      [2, 0, 2, 2],
      [0, 0, 0, 0],
      [4, 4, 8, 0],
      [2, 4, 8, 16],
    ]
    const original = grid.map((row) => [...row])

    expect(move2048(grid, 'right')).toEqual({
      grid: [
        [0, 0, 2, 4],
        [0, 0, 0, 0],
        [0, 0, 8, 8],
        [2, 4, 8, 16],
      ],
      score: 12,
      moved: true,
    })
    expect(grid).toEqual(original)
  })

  it.each([
    ['up', [[4, 0, 4, 0], [8, 0, 8, 0], [0, 0, 0, 0], [0, 0, 0, 0]]],
    ['down', [[0, 0, 0, 0], [0, 0, 0, 0], [4, 0, 4, 0], [8, 0, 8, 0]]],
  ] as const)('moves %s and scores vertical merges', (direction, expectedGrid) => {
    const grid: Grid2048 = [
      [2, 0, 2, 0],
      [2, 0, 2, 0],
      [4, 0, 4, 0],
      [4, 0, 4, 0],
    ]

    expect(move2048(grid, direction)).toEqual({
      grid: expectedGrid,
      score: 24,
      moved: true,
    })
  })

  it('reports a blocked move without manufacturing score', () => {
    const grid: Grid2048 = [
      [2, 4, 2, 4],
      [4, 2, 4, 2],
      [2, 4, 2, 4],
      [4, 2, 4, 2],
    ]

    expect(move2048(grid, 'left')).toEqual({ grid, score: 0, moved: false })
  })
})

describe('canMove2048', () => {
  it('accepts an empty cell or an adjacent equal pair', () => {
    expect(canMove2048([
      [2, 4, 8, 16],
      [32, 64, 128, 256],
      [512, 1024, 0, 2],
      [4, 8, 16, 32],
    ])).toBe(true)

    expect(canMove2048([
      [2, 4, 8, 16],
      [32, 64, 128, 256],
      [512, 1024, 2, 2],
      [4, 8, 16, 32],
    ])).toBe(true)
  })

  it('rejects a full grid with no adjacent equal tiles', () => {
    expect(canMove2048([
      [2, 4, 2, 4],
      [4, 2, 4, 2],
      [2, 4, 2, 4],
      [4, 2, 4, 2],
    ])).toBe(false)
  })
})
