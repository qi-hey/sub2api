<template>
  <AppLayout>
    <div v-if="game" class="game-player" :style="{ '--game-accent': game.accent, '--game-accent-soft': game.accentSoft }">
      <header class="game-player__header">
        <div class="game-player__identity">
          <button
            type="button"
            class="player-icon-button"
            :aria-label="t('gameCenter.actions.backToGames')"
            :title="t('gameCenter.actions.backToGames')"
            @click="backToGames"
          >
            <Icon name="arrowLeft" size="md" :stroke-width="2" />
          </button>
          <div class="game-player__title">
            <span>{{ t('gameCenter.playTitle') }}</span>
            <h1>{{ t(game.titleKey) }}</h1>
          </div>
        </div>

        <div class="game-player__scoreboard" aria-live="polite">
          <div>
            <span>{{ scoreLabel }}</span>
            <strong>{{ primaryScoreValue }}</strong>
          </div>
          <div>
            <span>{{ secondaryScoreLabel }}</span>
            <strong>{{ secondaryScoreValue }}</strong>
          </div>
          <div class="game-player__status">
            <span>{{ t('gameCenter.labels.status') }}</span>
            <strong><i></i>{{ statusLabel }}</strong>
          </div>
        </div>

        <div class="game-player__actions">
          <button
            type="button"
            class="player-icon-button"
            :aria-label="t('gameCenter.actions.restart')"
            :title="t('gameCenter.actions.restart')"
            @click="sendControl('restart')"
          >
            <Icon name="refresh" size="md" :stroke-width="2" />
          </button>
          <button
            type="button"
            class="player-icon-button player-icon-button--accent"
            :aria-label="pauseActionLabel"
            :title="pauseActionLabel"
            :aria-pressed="status === 'paused'"
            @click="sendControl('pause')"
          >
            <Icon v-if="status === 'paused'" name="play" size="md" :stroke-width="2" />
            <span v-else class="pause-glyph" aria-hidden="true"><i></i><i></i></span>
          </button>
        </div>
      </header>

      <div class="game-player__play-area" :class="{ 'game-player__play-area--slot': game.id === 'lucky' }">
        <div class="game-player__surface">
          <GameCanvas
            :key="game.id"
            ref="canvasRef"
            :game-id="game.id"
            :label="t(game.titleKey)"
            :controls-description="t(game.controlsKey)"
            :loading-label="t('common.loading')"
            :error-label="t('gameCenter.loadFailed')"
            :mute-label="t('gameCenter.actions.muteSound')"
            :unmute-label="t('gameCenter.actions.enableSound')"
            :get-slot-state="game.id === 'lucky' ? getSlotState : undefined"
            :request-slot-spin="game.id === 'lucky' ? requestSlotSpin : undefined"
            :request-slot-bonus="game.id === 'lucky' ? requestSlotBonus : undefined"
            :format-slot-error="game.id === 'lucky' ? formatSlotError : undefined"
            :translate="translateGameCopy"
            :prefers-reduced-motion="prefersReducedMotion"
            :get-fruit-state="game.id === 'merge2048' ? getFruitState : undefined"
            :request-fruit-spin="game.id === 'merge2048' ? requestFruitSpin : undefined"
            @score="handleScore"
            @status="handleStatus"
            @slot-result="handleSlotResult"
            @slot-error="handleSlotError"
            @slot-bonus-result="handleSlotBonusResult"
            @slot-bonus-error="handleSlotBonusError"
            @fruit-result="handleFruitResult"
            @fruit-error="handleFruitError"
          />
        </div>
        <SlotRulesPanel v-if="game.id === 'lucky'" />
      </div>

      <div class="mobile-controls" :aria-label="t('gameCenter.labels.touch')">
        <button
          v-for="control in game.mobileControls"
          :key="control"
          type="button"
          class="mobile-control"
          :class="{ 'mobile-control--accent': control === 'action' || control === 'pause' }"
          :aria-label="controlLabel(control)"
          :title="controlLabel(control)"
          :aria-pressed="control === 'pause' ? status === 'paused' : undefined"
          @click="sendControl(control)"
        >
          <Icon
            v-if="control !== 'pause' || status === 'paused'"
            :name="control === 'pause' ? 'play' : controlIcon(control)"
            size="lg"
            :stroke-width="2"
          />
          <span v-else class="pause-glyph" aria-hidden="true"><i></i><i></i></span>
        </button>

        <button
          type="button"
          class="mobile-control"
          :aria-label="t('gameCenter.actions.restart')"
          :title="t('gameCenter.actions.restart')"
          @click="sendControl('restart')"
        >
          <Icon name="refresh" size="lg" :stroke-width="2" />
        </button>
      </div>
    </div>

    <div v-else class="game-not-found">
      <span aria-hidden="true">404</span>
      <h1>{{ t('gameCenter.errors.notFoundTitle') }}</h1>
      <button type="button" class="game-not-found__back" @click="backToGames">
        <Icon name="arrowLeft" size="sm" :stroke-width="2" />
        {{ t('gameCenter.actions.backToGames') }}
      </button>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import GameCanvas from '@/features/game-center/components/GameCanvas.vue'
import SlotRulesPanel from '@/features/game-center/components/SlotRulesPanel.vue'
import { getGameById } from '@/features/game-center/catalog'
import { readBestScore, saveBestScore } from '@/features/game-center/storage'
import type {
  GameControl,
  GameStatus,
  SlotBonusServerResult,
  SlotSceneState,
  SlotSpinServerResult,
  FruitBets,
  FruitSceneState,
  FruitSpinServerResult,
} from '@/features/game-center/types'
import {
  claimGameWalletSlotBonus,
  createGameWalletIdempotencyKey,
  getGameWallet,
  spinGameWalletSlot,
  spinGameWalletFruit,
  submitGameLeaderboardScore,
} from '@/api/gameWallet'
import { useAppStore } from '@/stores'
import type { GameWallet } from '@/types'
import { extractApiErrorCode } from '@/utils/apiError'

type ControlIcon = 'arrowLeft' | 'arrowRight' | 'arrowUp' | 'arrowDown' | 'refresh' | 'play'

interface RetriableRequest<T> {
  idempotencyKey: string
  promise: Promise<T> | null
}

interface SlotBonusRequest extends RetriableRequest<SlotBonusServerResult> {
  choice: number
}

interface SlotSpinRequest extends RetriableRequest<SlotSpinServerResult> {
  betCredits?: string
}

interface FruitSpinRequest extends RetriableRequest<FruitSpinServerResult> {
  bets: FruitBets
}

interface LeaderboardState {
  highestScore: number
  submittedScore: number
}

const route = useRoute()
const router = useRouter()
const { locale, t } = useI18n()
const appStore = useAppStore()

const canvasRef = ref<InstanceType<typeof GameCanvas> | null>(null)
const score = ref(0)
const bestScore = ref(0)
const status = ref<GameStatus>('ready')
const slotWallet = ref<GameWallet | null>(null)
const slotWalletLoading = ref(false)
let slotWalletReloadQueued = false
let slotSpinRequest: SlotSpinRequest | null = null
let fruitSpinRequest: FruitSpinRequest | null = null
const slotBonusClaims = new Map<string, SlotBonusRequest>()
let scoreSubmitTimer: ReturnType<typeof setTimeout> | null = null
let scoreSubmission: Promise<void> | null = null
let queuedLeaderboardScore = 0
let lastSubmittedScore = 0
let scoreSubmissionGeneration = 0
let leaderboardGameId = ''
let leaderboardActive = true

const INSUFFICIENT_CREDITS_MESSAGE = '积分不足，无法旋转，请先签到获取积分'
const SCORE_SUBMIT_THROTTLE_MS = 1200
const LEADERBOARD_STORAGE_PREFIX = 'sub2api:game-center:leaderboard:'

const slotErrorKeys: Record<string, string> = {
  GAME_LOYALTY_DISABLED: 'gameCenter.apiErrors.disabled',
  GAME_LOYALTY_CONFIG_INVALID: 'gameCenter.apiErrors.configInvalid',
  GAME_LOYALTY_NOT_CONFIGURED: 'gameCenter.apiErrors.notConfigured',
  GAME_LOYALTY_CREDITS_LIMIT_EXCEEDED: 'gameCenter.apiErrors.creditsLimit',
  GAME_LOYALTY_BET_INVALID: 'gameCenter.apiErrors.invalidBet',
  GAME_LOYALTY_RNG_FAILED: 'gameCenter.apiErrors.rngFailed',
  GAME_LOYALTY_SPIN_LIMIT_EXCEEDED: 'gameCenter.apiErrors.spinLimit',
  GAME_FRUIT_BETS_INVALID: 'gameCenter.apiErrors.invalidBet',
  GAME_FRUIT_RETRY_BETS_MISMATCH: 'gameCenter.apiErrors.pendingFruitRetry',
  IDEMPOTENCY_KEY_REQUIRED: 'gameCenter.apiErrors.requestInvalid',
  IDEMPOTENCY_KEY_INVALID: 'gameCenter.apiErrors.requestInvalid',
  IDEMPOTENCY_KEY_CONFLICT: 'gameCenter.apiErrors.requestConflict',
}

const routeGameId = computed(() => {
  const value = route.params.id ?? route.params.gameId
  return Array.isArray(value) ? value[0] ?? '' : value ?? ''
})

const game = computed(() => getGameById(routeGameId.value))
const scoreLabel = computed(() => game.value?.id === 'lucky' || game.value?.id === 'merge2048'
  ? t('gameCenter.scores.credits')
  : t('gameCenter.labels.score'))
const primaryScoreValue = computed(() => game.value?.id === 'lucky' || game.value?.id === 'merge2048'
  ? formatCreditString(slotWallet.value?.credits ?? '0')
  : formatScore(score.value))
const secondaryScoreLabel = computed(() => game.value?.id === 'lucky'
  ? t('gameCenter.wallet.freeSpins')
  : game.value?.id === 'merge2048' ? t('gameCenter.labels.score')
  : t('gameCenter.labels.bestScore'))
const secondaryScoreValue = computed(() => game.value?.id === 'lucky'
  ? formatScore(slotWallet.value?.free_spins_remaining ?? 0)
  : game.value?.id === 'merge2048' ? formatScore(score.value)
  : formatScore(bestScore.value))
const statusLabel = computed(() => t(`gameCenter.status.${status.value}`))
const pauseActionLabel = computed(() => status.value === 'paused'
  ? t('gameCenter.actions.resume')
  : t('gameCenter.actions.pause'))

const iconByControl: Record<Exclude<GameControl, 'pause' | 'restart'>, ControlIcon> = {
  left: 'arrowLeft',
  right: 'arrowRight',
  up: 'arrowUp',
  down: 'arrowDown',
  rotate: 'refresh',
  action: 'play',
  drop: 'arrowDown',
}

watch(game, (nextGame, previousGame) => {
  if (previousGame?.id && previousGame.id !== 'lucky') void flushLeaderboardScore()
  resetScoreSubmission()
  score.value = 0
  status.value = 'ready'
  bestScore.value = nextGame ? readBestScore(nextGame.id) : 0
  slotWallet.value = null
  if (nextGame?.id === 'lucky' || nextGame?.id === 'merge2048') {
    void loadSlotWallet()
  } else if (nextGame) {
    initializeLeaderboardScore(nextGame.id)
  }
}, { immediate: true })

function controlIcon(control: GameControl): ControlIcon {
  if (control === 'pause' || control === 'restart') return control === 'pause' ? 'play' : 'refresh'
  return iconByControl[control]
}

function controlLabel(control: GameControl): string {
  if (control === 'pause') return pauseActionLabel.value
  if (control === 'restart') return t('gameCenter.actions.restart')
  return t(`gameCenter.controls.${control}`)
}

function formatScore(value: number): string {
  return new Intl.NumberFormat(locale.value).format(value)
}

function formatCreditString(value: string): string {
  if (!/^\d+$/.test(value)) return '--'
  try {
    return new Intl.NumberFormat(locale.value).format(BigInt(value))
  } catch {
    return '--'
  }
}

function getSlotState(): SlotSceneState {
  const pendingBonus = slotWallet.value?.pending_slot_bonus
  return {
    credits: slotWallet.value?.credits ?? '0',
    freeSpinsRemaining: slotWallet.value?.free_spins_remaining ?? 0,
    betCredits: slotWallet.value?.slot_bet_credits ?? '0',
    loyaltyEnabled: Boolean(slotWallet.value?.loyalty_enabled),
    loyaltyAvailable: Boolean(slotWallet.value?.loyalty_available) && !slotWalletLoading.value,
    ...(pendingBonus?.token
      ? {
          pendingSlotBonus: {
            token: pendingBonus.token,
            kind: pendingBonus.kind,
            expiresAt: pendingBonus.expires_at,
            betCredits: pendingBonus.bet_credits,
            bonusCount: pendingBonus.bonus_count,
            bonusMultiplier: pendingBonus.bonus_multiplier,
            options: pendingBonus.options,
          },
        }
      : {}),
  }
}

function getFruitState(): FruitSceneState {
  return {
    credits: slotWallet.value?.credits ?? '0',
    loyaltyEnabled: Boolean(slotWallet.value?.loyalty_enabled),
    loyaltyAvailable: Boolean(slotWallet.value?.loyalty_available) && !slotWalletLoading.value,
  }
}

async function loadSlotWallet(): Promise<void> {
  if (slotWalletLoading.value) {
    slotWalletReloadQueued = true
    return
  }
  slotWalletLoading.value = true
  try {
    slotWallet.value = await getGameWallet()
  } catch {
    appStore.showError(t('gameCenter.wallet.loadFailed'))
  } finally {
    slotWalletLoading.value = false
    if (slotWalletReloadQueued) {
      slotWalletReloadQueued = false
      void loadSlotWallet()
    } else {
      canvasRef.value?.syncSlotWallet()
      canvasRef.value?.syncWalletState?.()
    }
  }
}

function requestFruitSpin(bets: FruitBets): Promise<FruitSpinServerResult> {
  if (fruitSpinRequest?.promise) return fruitSpinRequest.promise
  if (fruitSpinRequest && !sameFruitBets(fruitSpinRequest.bets, bets)) {
    return Promise.reject(createFruitRetryBetsMismatchError(fruitSpinRequest.bets))
  }
  const requestState: FruitSpinRequest = fruitSpinRequest ?? {
    bets,
    idempotencyKey: createGameWalletIdempotencyKey(),
    promise: null,
  }
  fruitSpinRequest = requestState
  const promise = spinGameWalletFruit(requestState.bets, requestState.idempotencyKey).then(
    (result) => {
      if (fruitSpinRequest === requestState) fruitSpinRequest = null
      return result
    },
    (error: unknown) => {
      if (fruitSpinRequest === requestState) {
        requestState.promise = null
        if (isDefinitiveServerOutcome(error)) fruitSpinRequest = null
      }
      throw error
    },
  )
  requestState.promise = promise
  return promise
}

function sameFruitBets(left: FruitBets, right: FruitBets): boolean {
  const doors: Array<keyof FruitBets> = [
    'bar', '77', 'star', 'watermelon', 'bell', 'mango', 'orange', 'apple',
  ]
  return doors.every((door) => left[door] === right[door])
}

function createFruitRetryBetsMismatchError(pendingBets: FruitBets): unknown {
  return {
    code: 'GAME_FRUIT_RETRY_BETS_MISMATCH',
    reason: 'GAME_FRUIT_RETRY_BETS_MISMATCH',
    message: '上次开奖结果尚未确认，已恢复原下注，请再次点击启动',
    fruitRetryBets: { ...pendingBets },
  }
}

function requestSlotSpin(selectedBetCredits: string): Promise<SlotSpinServerResult> {
  if (slotSpinRequest?.promise) return slotSpinRequest.promise
  const state = getSlotState()
  const betCredits = state.freeSpinsRemaining > 0 ? undefined : selectedBetCredits
  if (slotSpinRequest && slotSpinRequest.betCredits !== betCredits) {
    return Promise.reject(new Error('上次旋转结果尚未确认，请使用原下注重试'))
  }
  if (!slotSpinRequest && !canAffordSlotSpin(state, betCredits ?? state.betCredits)) {
    const error = createInsufficientCreditsError(true)
    appStore.showError(INSUFFICIENT_CREDITS_MESSAGE)
    return Promise.reject(error)
  }

  const requestState = slotSpinRequest ?? {
    betCredits,
    idempotencyKey: createGameWalletIdempotencyKey(),
    promise: null,
  }
  slotSpinRequest = requestState
  const promise = spinGameWalletSlot(
    requestState.betCredits === undefined ? {} : { bet_credits: requestState.betCredits },
    requestState.idempotencyKey,
  ).then(
    (result) => {
      if (slotSpinRequest === requestState) slotSpinRequest = null
      return result
    },
    (error: unknown) => {
      if (slotSpinRequest === requestState) {
        requestState.promise = null
        if (isDefinitiveServerOutcome(error)) slotSpinRequest = null
      }
      throw error
    },
  )
  requestState.promise = promise
  return promise
}

function requestSlotBonus(token: string, choice: number): Promise<SlotBonusServerResult> {
  const existing = slotBonusClaims.get(token)
  if (existing?.promise) return existing.promise
  if (existing && existing.choice !== choice) {
    return Promise.reject(new Error('彩蛋奖励正在领取中'))
  }

  const requestState = existing ?? {
    choice,
    idempotencyKey: createGameWalletIdempotencyKey(),
    promise: null,
  }
  slotBonusClaims.set(token, requestState)
  const promise = claimGameWalletSlotBonus(
    { token, choice },
    requestState.idempotencyKey,
  ).then(
    (result) => {
      if (slotBonusClaims.get(token) === requestState) slotBonusClaims.delete(token)
      return result
    },
    (error: unknown) => {
      if (slotBonusClaims.get(token) === requestState) {
        requestState.promise = null
        if (isDefinitiveServerOutcome(error)) slotBonusClaims.delete(token)
      }
      throw error
    },
  )
  requestState.promise = promise
  return promise
}

function isDefinitiveServerOutcome(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const status = (error as { status?: unknown }).status
  return typeof status === 'number' && Number.isFinite(status) && status > 0
}

function handleSlotResult(result: SlotSpinServerResult): void {
  if (!slotWallet.value) return
  slotWallet.value = {
    ...slotWallet.value,
    credits: result.credits_after,
    free_spins_remaining: result.free_spins_remaining,
    ...(result.bonus_round ? { pending_slot_bonus: result.bonus_round } : {}),
  }
}

function handleSlotBonusResult(result: SlotBonusServerResult): void {
  if (slotWallet.value) {
    slotWallet.value = {
      ...slotWallet.value,
      credits: result.credits_after,
      pending_slot_bonus: undefined,
    }
  }
  void loadSlotWallet()
}

function handleFruitResult(result: FruitSpinServerResult): void {
  if (!slotWallet.value) return
  slotWallet.value = { ...slotWallet.value, credits: result.credits_after }
}

function handleFruitError(error: unknown): void {
  appStore.showError(formatSlotError(error))
  void loadSlotWallet()
}

function handleSlotError(error: unknown): void {
  if (!isSlotToastHandled(error)) appStore.showError(formatSlotError(error))
  void loadSlotWallet()
}

function handleSlotBonusError(error: unknown): void {
  appStore.showError(formatSlotError(error))
  void loadSlotWallet()
}

function formatSlotError(error: unknown): string {
  const code = extractApiErrorCode(error)
  if (code === 'GAME_LOYALTY_INSUFFICIENT_CREDITS') return INSUFFICIENT_CREDITS_MESSAGE
  const key = code ? slotErrorKeys[code] : undefined
  return t(key ?? 'gameCenter.slot.failed')
}

function canAffordSlotSpin(state: SlotSceneState, betCredits: string): boolean {
  if (state.freeSpinsRemaining > 0) return true
  const credits = parseUnsignedInteger(state.credits)
  const bet = parseUnsignedInteger(betCredits)
  return credits !== null && bet !== null && credits >= bet
}

function parseUnsignedInteger(value: string): bigint | null {
  if (!/^\d+$/.test(value)) return null
  try {
    return BigInt(value)
  } catch {
    return null
  }
}

function createInsufficientCreditsError(toastHandled = false): unknown {
  return {
    code: 'GAME_LOYALTY_INSUFFICIENT_CREDITS',
    reason: 'GAME_LOYALTY_INSUFFICIENT_CREDITS',
    message: INSUFFICIENT_CREDITS_MESSAGE,
    slotToastHandled: toastHandled,
  }
}

function isSlotToastHandled(error: unknown): boolean {
  return Boolean(error && typeof error === 'object'
    && (error as { slotToastHandled?: boolean }).slotToastHandled)
}

function translateGameCopy(key: string, params?: Record<string, string | number>): string {
  return t(key, params ?? {})
}

function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function handleScore(nextScore: number): void {
  if (!game.value) return
  score.value = nextScore
  bestScore.value = saveBestScore(game.value.id, nextScore)
  if (game.value.id !== 'lucky' && game.value.id !== 'merge2048') {
    queueLeaderboardScore(recordLeaderboardScore(game.value.id, nextScore))
  }
}

function handleStatus(nextStatus: GameStatus): void {
  status.value = nextStatus
  if (nextStatus === 'game-over' || nextStatus === 'complete') {
    void flushLeaderboardScore()
  }
}

function queueLeaderboardScore(nextScore: number): void {
  queuedLeaderboardScore = Math.max(queuedLeaderboardScore, Math.max(0, Math.floor(nextScore)))
  if (!leaderboardActive || queuedLeaderboardScore <= lastSubmittedScore || scoreSubmitTimer) return
  scoreSubmitTimer = setTimeout(() => {
    scoreSubmitTimer = null
    void flushLeaderboardScore()
  }, SCORE_SUBMIT_THROTTLE_MS)
}

async function flushLeaderboardScore(): Promise<void> {
  if (!leaderboardGameId || scoreSubmission) return
  if (scoreSubmitTimer) {
    clearTimeout(scoreSubmitTimer)
    scoreSubmitTimer = null
  }
  const gameId = leaderboardGameId
  const scoreToSubmit = queuedLeaderboardScore
  if (scoreToSubmit <= lastSubmittedScore) return

  const generation = scoreSubmissionGeneration
  const request = submitGameLeaderboardScore(gameId, scoreToSubmit)
    .then(() => {
      if (generation === scoreSubmissionGeneration) {
        lastSubmittedScore = Math.max(lastSubmittedScore, scoreToSubmit)
        persistLeaderboardState({
          highestScore: Math.max(queuedLeaderboardScore, scoreToSubmit),
          submittedScore: lastSubmittedScore,
        }, gameId)
      }
    })
    .catch(() => {
      // Keep the score pending. A later status change, timer, or visit retries it.
    })
    .finally(() => {
      if (scoreSubmission === request) scoreSubmission = null
      if (generation === scoreSubmissionGeneration && queuedLeaderboardScore > lastSubmittedScore) {
        queueLeaderboardScore(queuedLeaderboardScore)
      }
    })
  scoreSubmission = request
  await request
}

function initializeLeaderboardScore(gameId: string): void {
  const state = readLeaderboardState(gameId)
  leaderboardGameId = gameId
  queuedLeaderboardScore = Math.max(state.highestScore, game.value?.id === gameId ? bestScore.value : 0)
  lastSubmittedScore = Math.min(queuedLeaderboardScore, state.submittedScore)
  if (queuedLeaderboardScore > lastSubmittedScore) queueLeaderboardScore(queuedLeaderboardScore)
}

function recordLeaderboardScore(gameId: string, nextScore: number): number {
  if (leaderboardGameId !== gameId) {
    resetScoreSubmission()
    initializeLeaderboardScore(gameId)
  }

  const normalizedScore = Math.max(0, Math.floor(nextScore))
  queuedLeaderboardScore = Math.max(queuedLeaderboardScore, normalizedScore)
  persistLeaderboardState({
    highestScore: queuedLeaderboardScore,
    submittedScore: lastSubmittedScore,
  }, gameId)
  return queuedLeaderboardScore
}

function readLeaderboardState(gameId: string): LeaderboardState {
  const emptyState = { highestScore: 0, submittedScore: 0 }
  try {
    const raw = localStorage.getItem(leaderboardStorageKey(gameId))
    if (!raw) return emptyState
    const parsed = JSON.parse(raw) as Partial<LeaderboardState>
    const highestScore = normalizeStoredScore(parsed.highestScore)
    const submittedScore = Math.min(highestScore, normalizeStoredScore(parsed.submittedScore))
    return { highestScore, submittedScore }
  } catch {
    return emptyState
  }
}

function persistLeaderboardState(state: LeaderboardState, gameId: string): void {
  try {
    localStorage.setItem(leaderboardStorageKey(gameId), JSON.stringify(state))
  } catch {
    // In-memory submission still works when storage is unavailable.
  }
}

function leaderboardStorageKey(gameId: string): string {
  const version = gameId === 'merge2048' ? ':fruit-v1' : ''
  return `${LEADERBOARD_STORAGE_PREFIX}${encodeURIComponent(gameId)}${version}`
}

function normalizeStoredScore(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value)
    ? Math.max(0, Math.floor(value))
    : 0
}

function resetScoreSubmission(): void {
  scoreSubmissionGeneration += 1
  if (scoreSubmitTimer) clearTimeout(scoreSubmitTimer)
  scoreSubmitTimer = null
  scoreSubmission = null
  queuedLeaderboardScore = 0
  lastSubmittedScore = 0
  leaderboardGameId = ''
}

function sendControl(control: GameControl): void {
  canvasRef.value?.sendControl(control)
  canvasRef.value?.focus()
}

function backToGames(): void {
  void router.push('/games')
}

function handlePageExit(): void {
  void flushLeaderboardScore()
}

onMounted(() => {
  window.addEventListener('pagehide', handlePageExit)
})

onBeforeUnmount(() => {
  window.removeEventListener('pagehide', handlePageExit)
  void flushLeaderboardScore()
  leaderboardActive = false
  if (scoreSubmitTimer) clearTimeout(scoreSubmitTimer)
  scoreSubmitTimer = null
})
</script>

<style scoped>
.game-player {
  --game-accent: #22d3ee;
  --game-accent-soft: rgba(34, 211, 238, 0.2);
  width: 100%;
  max-width: 1380px;
  margin: 0 auto;
}

.game-player__header {
  display: grid;
  grid-template-columns: minmax(190px, 1fr) auto minmax(90px, 1fr);
  align-items: center;
  gap: 18px;
  margin-bottom: 16px;
}

.game-player__identity,
.game-player__actions {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.game-player__actions {
  justify-content: flex-end;
}

.player-icon-button,
.mobile-control {
  display: grid;
  flex: none;
  place-items: center;
  border: 1px solid rgba(100, 116, 139, 0.42);
  border-radius: 6px;
  background: #111827;
  color: #cbd5e1;
  cursor: pointer;
  touch-action: manipulation;
  transition: border-color 160ms ease, background-color 160ms ease, color 160ms ease, transform 160ms ease;
}

.player-icon-button {
  width: 42px;
  height: 42px;
}

.player-icon-button:hover,
.mobile-control:hover {
  border-color: var(--game-accent);
  color: var(--game-accent);
}

.player-icon-button:active,
.mobile-control:active {
  transform: translateY(1px);
}

.player-icon-button:focus-visible,
.mobile-control:focus-visible,
.game-not-found__back:focus-visible {
  outline: 3px solid color-mix(in srgb, var(--game-accent) 58%, white);
  outline-offset: 2px;
}

.player-icon-button--accent,
.mobile-control--accent {
  border-color: color-mix(in srgb, var(--game-accent) 60%, transparent);
  background: var(--game-accent-soft);
  color: var(--game-accent);
}

.game-player__title {
  min-width: 0;
}

.game-player__title span {
  display: block;
  margin-bottom: 2px;
  color: #64748b;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.2;
  text-transform: uppercase;
}

.game-player__title h1 {
  overflow: hidden;
  margin: 0;
  color: #111827;
  font-size: 20px;
  font-weight: 800;
  line-height: 1.25;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.dark) .game-player__title h1 {
  color: #f8fafc;
}

.game-player__scoreboard {
  display: grid;
  min-width: 340px;
  grid-template-columns: repeat(3, minmax(92px, 1fr));
  overflow: hidden;
  border: 1px solid rgba(100, 116, 139, 0.3);
  border-radius: 7px;
  background: #0b101a;
}

.game-player__scoreboard > div {
  display: grid;
  min-width: 0;
  gap: 3px;
  padding: 9px 14px;
  border-right: 1px solid rgba(100, 116, 139, 0.24);
}

.game-player__scoreboard > div:last-child {
  border-right: 0;
}

.game-player__scoreboard span {
  overflow: hidden;
  color: #64748b;
  font-size: 9px;
  font-weight: 700;
  line-height: 1.2;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}

.game-player__scoreboard strong {
  overflow: hidden;
  color: var(--game-accent);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 17px;
  font-weight: 800;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.game-player__status strong {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #cbd5e1;
  font-family: inherit;
  font-size: 12px;
}

.game-player__status i {
  width: 7px;
  height: 7px;
  flex: none;
  border-radius: 50%;
  background: var(--game-accent);
  box-shadow: 0 0 10px var(--game-accent);
}

.game-player__play-area {
  width: 100%;
  max-width: 1050px;
  margin: 0 auto;
}

.game-player__play-area--slot {
  display: grid;
  max-width: 1256px;
  grid-template-columns: minmax(0, 900px) minmax(300px, 340px);
  align-items: start;
  gap: 16px;
}

.game-player__surface {
  width: 100%;
  min-width: 0;
}

.pause-glyph {
  display: flex;
  width: 18px;
  height: 18px;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

.pause-glyph i {
  width: 4px;
  height: 15px;
  border-radius: 1px;
  background: currentColor;
}

.mobile-controls {
  display: none;
  width: 100%;
  max-width: 720px;
  grid-template-columns: repeat(auto-fit, minmax(52px, 1fr));
  gap: 9px;
  margin: 14px auto 0;
  padding: 10px;
  border: 1px solid rgba(100, 116, 139, 0.28);
  border-radius: 8px;
  background: rgba(11, 16, 26, 0.96);
}

.mobile-control {
  width: 100%;
  height: 52px;
}

.game-not-found {
  display: flex;
  min-height: min(520px, calc(100vh - 160px));
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.game-not-found > span {
  color: #22d3ee;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 56px;
  font-weight: 900;
  line-height: 1;
  text-shadow: 0 0 20px rgba(34, 211, 238, 0.35);
}

.game-not-found h1 {
  margin: 12px 0 22px;
  color: #111827;
  font-size: 21px;
  font-weight: 800;
  letter-spacing: 0;
}

:global(.dark) .game-not-found h1 {
  color: #f8fafc;
}

.game-not-found__back {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 40px;
  padding: 0 14px;
  border: 1px solid rgba(34, 211, 238, 0.45);
  border-radius: 6px;
  background: rgba(34, 211, 238, 0.12);
  color: #0891b2;
  font-weight: 700;
  cursor: pointer;
}

:global(.dark) .game-not-found__back {
  color: #67e8f9;
}

@media (max-width: 1023px) {
  .game-player__header {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .game-player__scoreboard {
    min-width: 0;
    grid-column: 1 / -1;
    grid-row: 2;
  }

  .mobile-controls {
    display: grid;
  }
}

@media (max-width: 899px) {
  .game-player__play-area--slot {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 639px) {
  .game-player__header {
    gap: 12px;
  }

  .game-player__actions {
    gap: 7px;
  }

  .player-icon-button {
    width: 38px;
    height: 38px;
  }

  .game-player__identity {
    gap: 8px;
  }

  .game-player__title h1 {
    max-width: 42vw;
    font-size: 16px;
  }

  .game-player__scoreboard {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .game-player__scoreboard > div {
    padding: 8px;
  }

  .game-player__scoreboard strong {
    font-size: 14px;
  }

  .game-player__status strong {
    font-size: 10px;
  }

  .mobile-controls {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

@media (prefers-reduced-motion: reduce) {
  .player-icon-button,
  .mobile-control {
    transition: none;
  }
}
</style>
