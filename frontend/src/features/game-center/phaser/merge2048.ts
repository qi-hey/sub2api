export type Grid2048 = number[][]

export interface Move2048Result {
  grid: Grid2048
  score: number
  moved: boolean
}

function mergeLine(line: number[]): { line: number[]; score: number } {
  const values = line.filter(Boolean)
  const merged: number[] = []
  let score = 0
  for (let index = 0; index < values.length; index += 1) {
    if (values[index] === values[index + 1]) {
      const value = values[index] * 2
      merged.push(value)
      score += value
      index += 1
    } else {
      merged.push(values[index])
    }
  }
  while (merged.length < line.length) merged.push(0)
  return { line: merged, score }
}

function transpose(grid: Grid2048): Grid2048 {
  return grid[0].map((_, column) => grid.map((row) => row[column]))
}

function reverseRows(grid: Grid2048): Grid2048 {
  return grid.map((row) => [...row].reverse())
}

function gridsEqual(left: Grid2048, right: Grid2048): boolean {
  return left.every((row, y) => row.every((value, x) => value === right[y][x]))
}

export function move2048(grid: Grid2048, direction: 'left' | 'right' | 'up' | 'down'): Move2048Result {
  const original = grid.map((row) => [...row])
  let working = original.map((row) => [...row])
  if (direction === 'right') working = reverseRows(working)
  if (direction === 'up') working = transpose(working)
  if (direction === 'down') working = reverseRows(transpose(working))

  let score = 0
  working = working.map((row) => {
    const result = mergeLine(row)
    score += result.score
    return result.line
  })

  if (direction === 'right') working = reverseRows(working)
  if (direction === 'up') working = transpose(working)
  if (direction === 'down') working = transpose(reverseRows(working))

  return { grid: working, score, moved: !gridsEqual(original, working) }
}

export function canMove2048(grid: Grid2048): boolean {
  if (grid.some((row) => row.some((value) => value === 0))) return true
  for (let y = 0; y < grid.length; y += 1) {
    for (let x = 0; x < grid[y].length; x += 1) {
      if (grid[y][x] === grid[y][x + 1] || grid[y][x] === grid[y + 1]?.[x]) return true
    }
  }
  return false
}
