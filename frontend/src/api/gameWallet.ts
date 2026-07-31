import { apiClient } from './client'
import type {
  GameWallet,
	GameFruitBets,
	GameFruitSpinResult,
  GameWalletCheckinResult,
  GameWalletRewardClaimRequest,
  GameWalletRewardClaimResult,
  GameWalletRewardList,
  GameWalletSpinRequest,
  GameWalletSpinResult,
  GameWalletTransactionList,
  GameWalletTransactionQuery,
} from '@/types'
import type {
	GameCreditGiftRequest,
	GameCreditGiftResult,
  GameLeaderboardQuery,
  GameLeaderboardResult,
  GameLeaderboardScoreResult,
  GameWalletSlotBonusClaimRequest,
  GameWalletSlotBonusClaimResult,
} from '@/types/gameWallet'

export function createGameWalletIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }

  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')

  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export async function getGameWallet(): Promise<GameWallet> {
  const { data } = await apiClient.get<GameWallet>('/user/game-wallet')
  return data
}

export async function checkInGameWallet(
  idempotencyKey = createGameWalletIdempotencyKey(),
): Promise<GameWalletCheckinResult> {
  const { data } = await apiClient.post<GameWalletCheckinResult>(
    '/user/game-wallet/check-in',
    {},
    {
      headers: {
        'Idempotency-Key': idempotencyKey,
      },
    },
  )
  return data
}

export async function giftGameWalletCredits(
	payload: GameCreditGiftRequest,
	idempotencyKey: string,
): Promise<GameCreditGiftResult> {
	const { data } = await apiClient.post<GameCreditGiftResult>(
		'/user/game-wallet/gifts',
		payload,
		{ headers: { 'Idempotency-Key': idempotencyKey } },
	)
	return data
}

export async function spinGameWalletSlot(
  payload: GameWalletSpinRequest,
  idempotencyKey: string,
): Promise<GameWalletSpinResult> {
  const { data } = await apiClient.post<GameWalletSpinResult>(
    '/user/game-wallet/slot/spin',
    payload,
    {
      headers: {
        'Idempotency-Key': idempotencyKey,
      },
    },
  )
  return data
}

export async function spinGameWalletFruit(
	bets: GameFruitBets,
	idempotencyKey: string,
): Promise<GameFruitSpinResult> {
	const { data } = await apiClient.post<GameFruitSpinResult>(
		'/user/game-wallet/fruit/spin',
		{ bets },
		{ headers: { 'Idempotency-Key': idempotencyKey } },
	)
	return data
}

export async function claimGameWalletSlotBonus(
  payload: GameWalletSlotBonusClaimRequest,
  idempotencyKey: string,
): Promise<GameWalletSlotBonusClaimResult> {
  const { data } = await apiClient.post<GameWalletSlotBonusClaimResult>(
    '/user/game-wallet/slot/bonus/claim',
    payload,
    {
      headers: {
        'Idempotency-Key': idempotencyKey,
      },
    },
  )
  return data
}

export async function getGameLeaderboard(
  gameIdOrQuery: string | GameLeaderboardQuery,
  limit = 20,
): Promise<GameLeaderboardResult> {
  const params = typeof gameIdOrQuery === 'string'
    ? { game_id: gameIdOrQuery, limit }
    : { game_id: gameIdOrQuery.game_id, limit: gameIdOrQuery.limit ?? limit }
  const { data } = await apiClient.get<GameLeaderboardResult>(
    '/user/game-wallet/leaderboard',
    { params },
  )
  return data
}

export async function submitGameLeaderboardScore(
  gameId: string,
  score: number,
): Promise<GameLeaderboardScoreResult> {
  const { data } = await apiClient.post<GameLeaderboardScoreResult>(
    '/user/game-wallet/leaderboard/score',
    { game_id: gameId, score },
  )
  return data
}

export async function getGameWalletRewards(): Promise<GameWalletRewardList> {
  const { data } = await apiClient.get<GameWalletRewardList>('/user/game-wallet/rewards')
  return data
}

export async function claimGameWalletReward(
  payload: GameWalletRewardClaimRequest,
  idempotencyKey = createGameWalletIdempotencyKey(),
): Promise<GameWalletRewardClaimResult> {
  const { data } = await apiClient.post<GameWalletRewardClaimResult>(
    '/user/game-wallet/rewards/claim',
    payload,
    {
      headers: {
        'Idempotency-Key': idempotencyKey,
      },
    },
  )
  return data
}

export async function getGameWalletTransactions(
  query: GameWalletTransactionQuery = {},
): Promise<GameWalletTransactionList> {
  const { data } = await apiClient.get<GameWalletTransactionList>(
    '/user/game-wallet/transactions',
    { params: query },
  )
  return data
}

export const gameWalletAPI = {
  getWallet: getGameWallet,
	checkIn: checkInGameWallet,
	giftCredits: giftGameWalletCredits,
  spin: spinGameWalletSlot,
	spinFruit: spinGameWalletFruit,
  claimSlotBonus: claimGameWalletSlotBonus,
  getLeaderboard: getGameLeaderboard,
  submitLeaderboardScore: submitGameLeaderboardScore,
  getRewards: getGameWalletRewards,
  claimReward: claimGameWalletReward,
  getTransactions: getGameWalletTransactions,
}

export default gameWalletAPI
