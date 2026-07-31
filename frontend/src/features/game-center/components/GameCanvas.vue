<template>
  <div
    ref="rootElement"
    class="game-canvas"
    :class="{
      'game-canvas--loading': loading,
      'game-canvas--error': Boolean(loadError),
      'game-canvas--lucky': gameId === 'lucky',
		'game-canvas--fruit': gameId === 'merge2048',
    }"
    :style="{ aspectRatio: gameId === 'lucky' ? '900 / 600' : '900 / 620' }"
    role="application"
    tabindex="0"
    :aria-label="label"
    :aria-describedby="controlsDescriptionId"
    :aria-busy="loading"
    @pointerdown="focusCanvas"
    @keydown="handleKeydown"
  >
    <span :id="controlsDescriptionId" class="sr-only">{{ controlsDescription }}</span>
    <div ref="mountElement" class="game-canvas__mount"></div>

    <button
      type="button"
      class="game-canvas__sound-toggle"
      :aria-label="soundToggleLabel"
      :title="soundToggleLabel"
      :aria-pressed="muted"
      @pointerdown.stop
      @click.stop="toggleSound"
    >
      <Icon :name="muted ? 'volumeOff' : 'volume'" size="md" :stroke-width="2" aria-hidden="true" />
    </button>

    <div v-if="loading" class="game-canvas__overlay" role="status">
      <span class="game-canvas__loader" aria-hidden="true"></span>
      <span class="sr-only">{{ loadingLabel }}</span>
    </div>

    <div v-else-if="loadError" class="game-canvas__overlay game-canvas__overlay--error" role="alert">
      <span class="game-canvas__error-mark" aria-hidden="true">!</span>
      <span>{{ errorLabel }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { ArcadeAudio, SOUND_EVENT, type ArcadeSound } from '../audio'
import type {
  GameControl,
  GameId,
  GameStatus,
  SceneHooks,
  SlotSceneState,
  SlotBonusServerResult,
  SlotSpinServerResult,
	FruitBets,
	FruitSceneState,
	FruitSpinServerResult,
} from '../types'

interface ArcadeGameInstance {
  events: {
    emit: (event: string, ...args: unknown[]) => boolean
    on: (event: string, handler: (...args: never[]) => void) => void
    off: (event: string, handler: (...args: never[]) => void) => void
  }
  destroy: (removeCanvas?: boolean, noReturn?: boolean) => void
  scene?: {
    getScenes?: (isActive?: boolean) => Array<{ syncWalletState?: () => void }>
  }
}

const props = withDefaults(defineProps<{
  gameId: GameId
  label: string
  controlsDescription: string
  loadingLabel?: string
  errorLabel?: string
  muteLabel?: string
  unmuteLabel?: string
  /** Slot only: live wallet snapshot for HUD. */
  getSlotState?: () => SlotSceneState
  /** Slot only: server spin request. */
  requestSlotSpin?: (betCredits: string) => Promise<SlotSpinServerResult>
  /** Slot only: server bonus claim request. */
  requestSlotBonus?: (token: string, choice: number) => Promise<SlotBonusServerResult>
  /** Slot only: shared business error formatter. */
  formatSlotError?: (error: unknown) => string
  /** Optional i18n bridge for Phaser scenes. */
  translate?: (key: string, params?: Record<string, string | number>) => string
  prefersReducedMotion?: () => boolean
	getFruitState?: () => FruitSceneState
	requestFruitSpin?: (bets: FruitBets) => Promise<FruitSpinServerResult>
}>(), {
  loadingLabel: '正在加载游戏',
  errorLabel: '游戏加载失败',
  muteLabel: '关闭声音',
  unmuteLabel: '开启声音',
  getSlotState: undefined,
  requestSlotSpin: undefined,
  requestSlotBonus: undefined,
  formatSlotError: undefined,
  translate: undefined,
  prefersReducedMotion: undefined,
	getFruitState: undefined,
	requestFruitSpin: undefined,
})

const emit = defineEmits<{
  score: [score: number]
  status: [status: GameStatus]
  ready: []
  error: [error: unknown]
  'slot-result': [result: SlotSpinServerResult]
  'slot-error': [error: unknown]
  'slot-bonus-result': [result: SlotBonusServerResult]
  'slot-bonus-error': [error: unknown]
	'fruit-result': [result: FruitSpinServerResult]
	'fruit-error': [error: unknown]
}>()

const mountElement = ref<HTMLElement | null>(null)
const rootElement = ref<HTMLElement | null>(null)
const loading = ref(true)
const loadError = ref<unknown>(null)
const audio = new ArcadeAudio()
const muted = ref(audio.isMuted)
const soundToggleLabel = computed(() => muted.value ? props.unmuteLabel : props.muteLabel)
const controlsDescriptionId = computed(() => `game-canvas-${props.gameId}-controls`)

const ARROW_KEYS = new Set(['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'])

let arcadeGame: ArcadeGameInstance | null = null
let controlEvent = 'sub2api-game-control'
let loadGeneration = 0

function destroyGame(): void {
  arcadeGame?.events.off(SOUND_EVENT, handleSound as (...args: never[]) => void)
  arcadeGame?.destroy(true)
  arcadeGame = null
}

function handleSound(sound: ArcadeSound): void {
  audio.play(sound)
}

const FRUIT_SYNC_SOUNDS: readonly ArcadeSound[] = ['fruit-center-spin', 'fruit-outer-spin']

async function createGame(): Promise<void> {
  const generation = ++loadGeneration
  destroyGame()
  loading.value = true
  loadError.value = null
  if (props.gameId === 'merge2048') audio.preload(FRUIT_SYNC_SOUNDS)
  await nextTick()

  if (!mountElement.value || generation !== loadGeneration) return

  try {
    const [{ createArcadeGame }, { CONTROL_EVENT }] = await Promise.all([
      import('../phaser/createArcadeGame'),
      import('../phaser/ArcadeScene'),
    ])

    if (!mountElement.value || generation !== loadGeneration) return

    controlEvent = CONTROL_EVENT
    const hooks: SceneHooks = {
      onScore: (score) => emit('score', score),
      onStatus: (status) => emit('status', status),
    }
    if (props.gameId === 'lucky') {
      if (props.getSlotState) hooks.getSlotState = props.getSlotState
      if (props.requestSlotSpin) hooks.requestSlotSpin = props.requestSlotSpin
      if (props.requestSlotBonus) hooks.requestSlotBonus = props.requestSlotBonus
      hooks.onSlotResult = (result) => emit('slot-result', result)
      hooks.onSlotError = (error) => emit('slot-error', error)
      hooks.onSlotBonusResult = (result) => emit('slot-bonus-result', result)
      hooks.onSlotBonusError = (error) => emit('slot-bonus-error', error)
      if (props.formatSlotError) hooks.formatSlotError = props.formatSlotError
      if (props.translate) hooks.t = props.translate
      if (props.prefersReducedMotion) hooks.prefersReducedMotion = props.prefersReducedMotion
    }
	if (props.gameId === 'merge2048') {
		if (props.getFruitState) hooks.getFruitState = props.getFruitState
		if (props.requestFruitSpin) hooks.requestFruitSpin = props.requestFruitSpin
		hooks.onFruitResult = (result) => emit('fruit-result', result)
		hooks.onFruitError = (error) => emit('fruit-error', error)
		if (props.formatSlotError) hooks.formatSlotError = props.formatSlotError
			if (props.prefersReducedMotion) hooks.prefersReducedMotion = props.prefersReducedMotion
			hooks.playSynchronizedSound = (sound) => audio.playSynchronized(sound)
		}
    arcadeGame = createArcadeGame(mountElement.value, props.gameId, hooks) as ArcadeGameInstance
    arcadeGame.events.on(SOUND_EVENT, handleSound as (...args: never[]) => void)
    loading.value = false
    emit('ready')
  } catch (error: unknown) {
    if (generation !== loadGeneration) return
    loading.value = false
    loadError.value = error
    emit('error', error)
  }
}

function sendControl(control: GameControl): void {
  void audio.unlock()
  arcadeGame?.events.emit(controlEvent, control)
}

function toggleSound(): void {
  muted.value = !muted.value
  audio.setMuted(muted.value)
  if (!muted.value) void audio.unlock()
}

function focusCanvas(): void {
  rootElement.value?.focus()
}

function handleKeydown(event: KeyboardEvent): void {
  if (ARROW_KEYS.has(event.key)) {
    event.preventDefault()
    return
  }

  if (event.code !== 'KeyP' && event.key.toLowerCase() !== 'p') return

  event.preventDefault()
  event.stopPropagation()
  if (!event.repeat) sendControl('pause')
}

watch(() => props.gameId, () => {
  void createGame()
})

onMounted(() => {
  if (rootElement.value) audio.attachUnlockListeners(rootElement.value)
  void createGame()
})

onBeforeUnmount(() => {
  loadGeneration += 1
  destroyGame()
  audio.destroy()
})

function syncSlotWallet(): void {
  const scenes = arcadeGame?.scene?.getScenes?.(true) ?? []
  for (const scene of scenes) {
    scene.syncWalletState?.()
  }
}

function syncWalletState(): void {
	const scenes = arcadeGame?.scene?.getScenes?.(true) ?? []
	for (const scene of scenes) scene.syncWalletState?.()
}

function playUiSound(sound: ArcadeSound): void {
  void audio.unlock()
  audio.play(sound)
}

defineExpose({
  sendControl,
  focus: focusCanvas,
  toggleSound,
  syncSlotWallet,
	syncWalletState,
  playUiSound,
  get isMuted() {
    return muted.value
  },
})
</script>

<style scoped>
.game-canvas {
  position: relative;
  width: 100%;
  overflow: hidden;
  aspect-ratio: 900 / 620;
  border: 1px solid rgba(34, 211, 238, 0.34);
  border-radius: 8px;
  background: #05070d;
  box-shadow: 0 18px 46px rgba(2, 6, 23, 0.3), 0 0 28px rgba(34, 211, 238, 0.08);
  outline: none;
  isolation: isolate;
}

.game-canvas:focus-visible {
  border-color: #67e8f9;
  box-shadow: 0 0 0 3px rgba(34, 211, 238, 0.24), 0 18px 46px rgba(2, 6, 23, 0.3);
}

.game-canvas--lucky {
  border-color: rgba(251, 191, 36, 0.72);
  background: #2a0b52;
  box-shadow: 0 18px 46px rgba(42, 11, 82, 0.32), 0 0 24px rgba(245, 158, 11, 0.16);
}

.game-canvas--lucky:focus-visible {
  border-color: #fde68a;
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.3), 0 18px 46px rgba(42, 11, 82, 0.32);
}

.game-canvas--fruit {
	border-color: rgba(227, 179, 65, 0.8);
	background: #250608;
}

.game-canvas--fruit .game-canvas__mount :deep(canvas) {
	touch-action: none;
	user-select: none;
	-webkit-touch-callout: none;
}

.game-canvas--lucky .game-canvas__sound-toggle {
  border-color: rgba(251, 191, 36, 0.7);
  background: rgba(107, 21, 38, 0.94);
  color: #fff7c2;
}

.game-canvas__mount {
  position: absolute;
  inset: 0;
}

.game-canvas__mount :deep(canvas) {
  display: block;
  max-width: 100%;
  max-height: 100%;
  outline: none;
}

.game-canvas__sound-toggle {
  position: absolute;
  z-index: 3;
  top: 12px;
  right: 12px;
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border: 1px solid rgba(100, 116, 139, 0.56);
  border-radius: 6px;
  background: rgba(11, 16, 26, 0.92);
  color: #e2e8f0;
  cursor: pointer;
  font-size: 18px;
  line-height: 1;
  touch-action: manipulation;
}

.game-canvas__sound-toggle:hover,
.game-canvas__sound-toggle:focus-visible {
  border-color: #67e8f9;
}

.game-canvas__sound-toggle:focus-visible {
  outline: 3px solid rgba(34, 211, 238, 0.3);
  outline-offset: 2px;
}

.game-canvas__overlay {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: grid;
  place-items: center;
  background: #05070d;
  color: #cbd5e1;
}

.game-canvas__loader {
  width: 34px;
  height: 34px;
  border: 2px solid rgba(34, 211, 238, 0.18);
  border-top-color: #22d3ee;
  border-radius: 50%;
  animation: game-canvas-spin 720ms linear infinite;
  box-shadow: 0 0 17px rgba(34, 211, 238, 0.22);
}

.game-canvas__overlay--error {
  display: flex;
  flex-direction: column;
  gap: 10px;
  color: #fda4af;
  font-size: 13px;
  font-weight: 700;
}

.game-canvas__error-mark {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid #fb7185;
  border-radius: 50%;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 18px;
}

@keyframes game-canvas-spin {
  to { transform: rotate(360deg); }
}

@media (prefers-reduced-motion: reduce) {
  .game-canvas__loader {
    animation: none;
    border-color: rgba(34, 211, 238, 0.45);
  }
}
</style>
