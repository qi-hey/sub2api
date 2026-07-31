import type { GameId } from './types'

const SCORE_PREFIX = 'sub2api.game-center.best.'

function scoreKey(gameId: GameId): string {
  return `${SCORE_PREFIX}${gameId}${gameId === 'merge2048' ? ':fruit-v1' : ''}`
}

export function readBestScore(gameId: GameId): number {
  const stored = Number(window.localStorage.getItem(scoreKey(gameId)))
  return Number.isFinite(stored) && stored > 0 ? Math.floor(stored) : 0
}

export function saveBestScore(gameId: GameId, score: number): number {
  const normalized = Math.max(0, Math.floor(score))
  const best = Math.max(readBestScore(gameId), normalized)
  window.localStorage.setItem(scoreKey(gameId), String(best))
  return best
}
