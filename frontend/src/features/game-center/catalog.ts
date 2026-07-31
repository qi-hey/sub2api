import type { GameCatalogItem, GameId } from './types'

export const GAME_CATALOG: GameCatalogItem[] = [
  {
    id: 'snake',
    accent: '#4ade80',
    accentSoft: 'rgba(74, 222, 128, 0.2)',
    titleKey: 'gameCenter.games.snake.title',
    subtitleKey: 'gameCenter.games.snake.subtitle',
    descriptionKey: 'gameCenter.games.snake.description',
    controlsKey: 'gameCenter.games.snake.controls',
    mobileControls: ['left', 'up', 'down', 'right', 'pause'],
  },
  {
    id: 'tetris',
    accent: '#22d3ee',
    accentSoft: 'rgba(34, 211, 238, 0.2)',
    titleKey: 'gameCenter.games.tetris.title',
    subtitleKey: 'gameCenter.games.tetris.subtitle',
    descriptionKey: 'gameCenter.games.tetris.description',
    controlsKey: 'gameCenter.games.tetris.controls',
    mobileControls: ['left', 'rotate', 'right', 'down', 'drop', 'pause'],
  },
  {
    id: 'breakout',
    accent: '#f472b6',
    accentSoft: 'rgba(244, 114, 182, 0.2)',
    titleKey: 'gameCenter.games.breakout.title',
    subtitleKey: 'gameCenter.games.breakout.subtitle',
    descriptionKey: 'gameCenter.games.breakout.description',
    controlsKey: 'gameCenter.games.breakout.controls',
    mobileControls: ['left', 'action', 'right', 'pause'],
  },
  {
    id: 'merge2048',
    accent: '#e3b341',
    accentSoft: 'rgba(31, 139, 82, 0.2)',
    titleKey: 'gameCenter.games.merge2048.title',
    subtitleKey: 'gameCenter.games.merge2048.subtitle',
    descriptionKey: 'gameCenter.games.merge2048.description',
    controlsKey: 'gameCenter.games.merge2048.controls',
    mobileControls: ['action', 'pause'],
  },
  {
    id: 'orbital',
    accent: '#38bdf8',
    accentSoft: 'rgba(56, 189, 248, 0.2)',
    titleKey: 'gameCenter.games.orbital.title',
    subtitleKey: 'gameCenter.games.orbital.subtitle',
    descriptionKey: 'gameCenter.games.orbital.description',
    controlsKey: 'gameCenter.games.orbital.controls',
    mobileControls: ['left', 'action', 'right', 'pause'],
  },
  {
    id: 'hoarder',
    accent: '#a3e635',
    accentSoft: 'rgba(163, 230, 53, 0.2)',
    titleKey: 'gameCenter.games.hoarder.title',
    subtitleKey: 'gameCenter.games.hoarder.subtitle',
    descriptionKey: 'gameCenter.games.hoarder.description',
    controlsKey: 'gameCenter.games.hoarder.controls',
    mobileControls: ['action', 'pause'],
  },
  {
    id: 'lucky',
    accent: '#f59e0b',
    accentSoft: 'rgba(245, 158, 11, 0.2)',
    titleKey: 'gameCenter.games.lucky.title',
    subtitleKey: 'gameCenter.games.lucky.subtitle',
    descriptionKey: 'gameCenter.games.lucky.description',
    controlsKey: 'gameCenter.games.lucky.controls',
    mobileControls: ['action', 'pause'],
  },
]

export function getGameById(id: string): GameCatalogItem | undefined {
  return GAME_CATALOG.find((game) => game.id === id)
}

export function isGameId(id: string): id is GameId {
  return GAME_CATALOG.some((game) => game.id === id)
}
