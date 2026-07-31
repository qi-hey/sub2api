import type { GameAudioSyncAnchor } from './types'

export const SOUND_EVENT = 'sub2api-game-sound'
export const AUDIO_MUTED_STORAGE_KEY = 'sub2api-game-audio-muted'

export type ArcadeSound =
  | 'move'
  | 'collect'
  | 'game-over'
  | 'complete'
  | 'tetris-rotate'
  | 'tetris-drop'
  | 'tetris-line-clear'
  | 'breakout-launch'
  | 'breakout-brick'
  | 'breakout-life'
  | 'orbital-shoot'
  | 'orbital-hit'
  | 'orbital-life'
  | 'hoarder-hit'
  | 'hoarder-countdown-end'
  | 'slots-reel-tick'
  | 'slots-spin'
  | 'slots-reel-stop'
  | 'slots-win'
  | 'slots-big-win'
  | 'slots-jackpot'
  | 'slots-no-win'
  | 'slots-free-spins'
  | 'slots-bonus-enter'
  | 'slots-bonus-pick'
  | 'slots-bonus-wheel'
  | 'slots-bonus-reveal'
  | 'slots-bonus-land'
  | 'fruit-center-spin'
  | 'fruit-outer-spin'
  | 'fruit-reel-stop'
  | 'fruit-center-stop'
  | 'fruit-win'
  | 'fruit-jackpot'
  | 'fruit-no-win'
  | 'fruit-bonus-enter'
  | 'fruit-bonus-land'
  | 'checkin-success'
  | 'reward-success'

type StorageLike = Pick<Storage, 'getItem' | 'setItem'>

interface Tone {
  frequency: number
  endFrequency?: number
  offset?: number
  duration: number
  volume: number
  type?: OscillatorType
}

interface ArcadeAudioOptions {
  storage?: StorageLike | null
  createContext?: () => AudioContext
  now?: () => number
  createMediaElement?: ((source: string) => HTMLAudioElement) | null
}

interface MediaClip {
  source: string
  volume: number
  durationMs?: number
}

const FRUIT_CENTER_FALLBACK: readonly Tone[] = Array.from({ length: 40 }, (_, index) => ({
  frequency: [392, 523, 659, 784][index % 4],
  offset: index * 0.16,
  duration: 0.075,
  volume: 0.042,
  type: index % 2 === 0 ? 'square' : 'triangle',
}))

const FRUIT_OUTER_FALLBACK: readonly Tone[] = Array.from({ length: 48 }, (_, index) => ({
  frequency: index < 36 ? 1175 : Math.max(440, 1175 - (index - 35) * 58),
  offset: index * (index < 36 ? 0.105 : 0.125),
  duration: 0.036,
  volume: 0.038,
  type: 'triangle',
}))

const MEDIA_CLIPS: Partial<Record<ArcadeSound, MediaClip>> = {
  'slots-spin': { source: `${import.meta.env.BASE_URL}game-audio/lucky-spin.mp3`, volume: 0.52 },
  'slots-win': { source: `${import.meta.env.BASE_URL}game-audio/lucky-win.mp3`, volume: 0.56 },
  'slots-big-win': { source: `${import.meta.env.BASE_URL}game-audio/lucky-jackpot.mp3`, volume: 0.6 },
  'slots-jackpot': { source: `${import.meta.env.BASE_URL}game-audio/lucky-jackpot.mp3`, volume: 0.66 },
  'fruit-center-spin': { source: `${import.meta.env.BASE_URL}game-audio/fruit-center.mp3`, volume: 0.58, durationMs: 7300 },
  'fruit-outer-spin': { source: `${import.meta.env.BASE_URL}game-audio/fruit-outer.mp3`, volume: 0.6, durationMs: 5900 },
  'fruit-win': { source: `${import.meta.env.BASE_URL}game-audio/fruit-win.mp3`, volume: 0.62, durationMs: 2250 },
  'fruit-jackpot': { source: `${import.meta.env.BASE_URL}game-audio/fruit-win.mp3`, volume: 0.7, durationMs: 2250 },
}

const SOUND_PATTERNS: Record<ArcadeSound, readonly Tone[]> = {
  move: [{ frequency: 170, endFrequency: 215, duration: 0.025, volume: 0.035, type: 'square' }],
  collect: [
    { frequency: 520, endFrequency: 650, duration: 0.06, volume: 0.08, type: 'sine' },
    { frequency: 780, offset: 0.045, duration: 0.08, volume: 0.07, type: 'sine' },
  ],
  'game-over': [
    { frequency: 240, endFrequency: 150, duration: 0.2, volume: 0.09, type: 'sawtooth' },
    { frequency: 145, endFrequency: 65, offset: 0.16, duration: 0.32, volume: 0.1, type: 'triangle' },
  ],
  complete: [
    { frequency: 523, duration: 0.11, volume: 0.08, type: 'triangle' },
    { frequency: 659, offset: 0.09, duration: 0.12, volume: 0.08, type: 'triangle' },
    { frequency: 784, offset: 0.18, duration: 0.2, volume: 0.09, type: 'triangle' },
  ],
  'tetris-rotate': [{ frequency: 280, endFrequency: 460, duration: 0.055, volume: 0.055, type: 'square' }],
  'tetris-drop': [{ frequency: 130, endFrequency: 55, duration: 0.12, volume: 0.09, type: 'triangle' }],
  'tetris-line-clear': [
    { frequency: 420, duration: 0.08, volume: 0.075, type: 'square' },
    { frequency: 620, offset: 0.055, duration: 0.09, volume: 0.075, type: 'square' },
    { frequency: 880, offset: 0.11, duration: 0.13, volume: 0.08, type: 'square' },
  ],
  'breakout-launch': [{ frequency: 180, endFrequency: 480, duration: 0.11, volume: 0.07, type: 'sine' }],
  'breakout-brick': [{ frequency: 690, endFrequency: 330, duration: 0.04, volume: 0.055, type: 'square' }],
  'breakout-life': [{ frequency: 190, endFrequency: 82, duration: 0.22, volume: 0.09, type: 'triangle' }],
  'orbital-shoot': [{ frequency: 820, endFrequency: 290, duration: 0.065, volume: 0.045, type: 'sawtooth' }],
  'orbital-hit': [{ frequency: 170, endFrequency: 70, duration: 0.08, volume: 0.065, type: 'square' }],
  'orbital-life': [
    { frequency: 180, endFrequency: 90, duration: 0.16, volume: 0.09, type: 'sawtooth' },
    { frequency: 75, offset: 0.12, duration: 0.18, volume: 0.08, type: 'triangle' },
  ],
  'hoarder-hit': [{ frequency: 210, endFrequency: 560, duration: 0.075, volume: 0.075, type: 'triangle' }],
  'hoarder-countdown-end': [
    { frequency: 880, duration: 0.07, volume: 0.075, type: 'square' },
    { frequency: 880, offset: 0.1, duration: 0.07, volume: 0.075, type: 'square' },
    { frequency: 440, offset: 0.2, duration: 0.18, volume: 0.09, type: 'square' },
  ],
  'slots-reel-tick': [{ frequency: 1175, endFrequency: 1397, duration: 0.022, volume: 0.026, type: 'triangle' }],
  'slots-spin': [
    { frequency: 392, endFrequency: 784, duration: 0.14, volume: 0.045, type: 'triangle' },
    { frequency: 523, offset: 0.025, duration: 0.075, volume: 0.042, type: 'sine' },
    { frequency: 659, offset: 0.075, duration: 0.075, volume: 0.044, type: 'sine' },
    { frequency: 784, offset: 0.125, duration: 0.085, volume: 0.047, type: 'sine' },
    { frequency: 1047, offset: 0.185, duration: 0.095, volume: 0.052, type: 'triangle' },
    { frequency: 1319, offset: 0.245, duration: 0.095, volume: 0.046, type: 'sine' },
    { frequency: 1568, offset: 0.305, duration: 0.16, volume: 0.052, type: 'triangle' },
  ],
  'slots-reel-stop': [
    { frequency: 784, endFrequency: 659, duration: 0.045, volume: 0.04, type: 'triangle' },
    { frequency: 1047, offset: 0.018, duration: 0.055, volume: 0.035, type: 'sine' },
    { frequency: 1319, offset: 0.042, duration: 0.065, volume: 0.03, type: 'sine' },
  ],
  'slots-win': [
    { frequency: 523, duration: 0.09, volume: 0.055, type: 'triangle' },
    { frequency: 1047, duration: 0.07, volume: 0.025, type: 'sine' },
    { frequency: 659, offset: 0.06, duration: 0.09, volume: 0.057, type: 'triangle' },
    { frequency: 1319, offset: 0.06, duration: 0.07, volume: 0.025, type: 'sine' },
    { frequency: 784, offset: 0.12, duration: 0.1, volume: 0.06, type: 'triangle' },
    { frequency: 1568, offset: 0.12, duration: 0.08, volume: 0.026, type: 'sine' },
    { frequency: 988, offset: 0.18, duration: 0.1, volume: 0.062, type: 'triangle' },
    { frequency: 1976, offset: 0.18, duration: 0.08, volume: 0.026, type: 'sine' },
    { frequency: 1319, offset: 0.25, duration: 0.23, volume: 0.068, type: 'triangle' },
    { frequency: 2637, offset: 0.25, duration: 0.15, volume: 0.024, type: 'sine' },
  ],
  'slots-big-win': [
    { frequency: 392, duration: 0.1, volume: 0.052, type: 'triangle' },
    { frequency: 784, duration: 0.08, volume: 0.024, type: 'sine' },
    { frequency: 523, offset: 0.065, duration: 0.1, volume: 0.056, type: 'triangle' },
    { frequency: 1047, offset: 0.065, duration: 0.08, volume: 0.024, type: 'sine' },
    { frequency: 659, offset: 0.13, duration: 0.11, volume: 0.059, type: 'triangle' },
    { frequency: 1319, offset: 0.13, duration: 0.085, volume: 0.025, type: 'sine' },
    { frequency: 784, offset: 0.195, duration: 0.12, volume: 0.062, type: 'triangle' },
    { frequency: 1568, offset: 0.195, duration: 0.09, volume: 0.026, type: 'sine' },
    { frequency: 1047, offset: 0.27, duration: 0.14, volume: 0.067, type: 'triangle' },
    { frequency: 1319, offset: 0.36, duration: 0.14, volume: 0.069, type: 'triangle' },
    { frequency: 1568, offset: 0.45, duration: 0.32, volume: 0.074, type: 'sine' },
    { frequency: 2093, offset: 0.45, duration: 0.22, volume: 0.034, type: 'triangle' },
  ],
  'slots-jackpot': [
    { frequency: 392, duration: 0.12, volume: 0.05, type: 'triangle' },
    { frequency: 784, duration: 0.1, volume: 0.024, type: 'sine' },
    { frequency: 523, offset: 0.07, duration: 0.12, volume: 0.055, type: 'triangle' },
    { frequency: 1047, offset: 0.07, duration: 0.1, volume: 0.024, type: 'sine' },
    { frequency: 659, offset: 0.14, duration: 0.12, volume: 0.055, type: 'triangle' },
    { frequency: 1319, offset: 0.14, duration: 0.1, volume: 0.024, type: 'sine' },
    { frequency: 784, offset: 0.21, duration: 0.14, volume: 0.06, type: 'triangle' },
    { frequency: 1568, offset: 0.21, duration: 0.1, volume: 0.025, type: 'sine' },
    { frequency: 1047, offset: 0.29, duration: 0.18, volume: 0.065, type: 'triangle' },
    { frequency: 1319, offset: 0.38, duration: 0.18, volume: 0.065, type: 'triangle' },
    { frequency: 1568, offset: 0.47, duration: 0.18, volume: 0.07, type: 'triangle' },
    { frequency: 2093, offset: 0.56, duration: 0.42, volume: 0.075, type: 'sine' },
    { frequency: 1047, offset: 0.56, duration: 0.42, volume: 0.035, type: 'triangle' },
    { frequency: 1319, offset: 0.72, duration: 0.16, volume: 0.062, type: 'triangle' },
    { frequency: 1568, offset: 0.82, duration: 0.16, volume: 0.067, type: 'triangle' },
    { frequency: 2093, offset: 0.92, duration: 0.5, volume: 0.08, type: 'sine' },
    { frequency: 2637, offset: 0.92, duration: 0.36, volume: 0.032, type: 'triangle' },
  ],
  'slots-no-win': [
    { frequency: 523, endFrequency: 494, duration: 0.07, volume: 0.035, type: 'triangle' },
    { frequency: 392, offset: 0.065, duration: 0.09, volume: 0.035, type: 'sine' },
  ],
  'slots-free-spins': [
    { frequency: 587, offset: 0.24, duration: 0.1, volume: 0.05, type: 'triangle' },
    { frequency: 1175, offset: 0.24, duration: 0.08, volume: 0.022, type: 'sine' },
    { frequency: 740, offset: 0.31, duration: 0.1, volume: 0.052, type: 'triangle' },
    { frequency: 1480, offset: 0.31, duration: 0.08, volume: 0.022, type: 'sine' },
    { frequency: 880, offset: 0.38, duration: 0.12, volume: 0.055, type: 'triangle' },
    { frequency: 1760, offset: 0.38, duration: 0.09, volume: 0.023, type: 'sine' },
    { frequency: 1175, offset: 0.47, duration: 0.24, volume: 0.065, type: 'triangle' },
    { frequency: 1760, offset: 0.47, duration: 0.24, volume: 0.045, type: 'sine' },
  ],
  'slots-bonus-enter': [
    { frequency: 392, duration: 0.11, volume: 0.052, type: 'triangle' },
    { frequency: 784, duration: 0.08, volume: 0.022, type: 'sine' },
    { frequency: 523, offset: 0.07, duration: 0.11, volume: 0.055, type: 'triangle' },
    { frequency: 1047, offset: 0.07, duration: 0.08, volume: 0.022, type: 'sine' },
    { frequency: 659, offset: 0.14, duration: 0.12, volume: 0.058, type: 'triangle' },
    { frequency: 1319, offset: 0.14, duration: 0.09, volume: 0.024, type: 'sine' },
    { frequency: 784, offset: 0.21, duration: 0.13, volume: 0.06, type: 'triangle' },
    { frequency: 1047, offset: 0.3, duration: 0.28, volume: 0.07, type: 'sine' },
    { frequency: 1568, offset: 0.3, duration: 0.28, volume: 0.04, type: 'triangle' },
  ],
  'slots-bonus-pick': [
    { frequency: 659, endFrequency: 988, duration: 0.09, volume: 0.045, type: 'triangle' },
    { frequency: 1319, offset: 0.035, duration: 0.08, volume: 0.026, type: 'sine' },
    { frequency: 1976, offset: 0.08, duration: 0.12, volume: 0.03, type: 'sine' },
  ],
  'slots-bonus-wheel': [
    { frequency: 262, endFrequency: 1047, duration: 0.32, volume: 0.045, type: 'triangle' },
    { frequency: 523, offset: 0.08, duration: 0.07, volume: 0.035, type: 'sine' },
    { frequency: 659, offset: 0.14, duration: 0.07, volume: 0.035, type: 'sine' },
    { frequency: 784, offset: 0.2, duration: 0.07, volume: 0.038, type: 'sine' },
    { frequency: 1047, offset: 0.27, duration: 0.15, volume: 0.045, type: 'sine' },
  ],
  'slots-bonus-reveal': [
    { frequency: 659, duration: 0.1, volume: 0.052, type: 'triangle' },
    { frequency: 1319, duration: 0.08, volume: 0.024, type: 'sine' },
    { frequency: 784, offset: 0.07, duration: 0.11, volume: 0.055, type: 'triangle' },
    { frequency: 1568, offset: 0.07, duration: 0.08, volume: 0.024, type: 'sine' },
    { frequency: 988, offset: 0.14, duration: 0.12, volume: 0.06, type: 'triangle' },
    { frequency: 1976, offset: 0.14, duration: 0.09, volume: 0.025, type: 'sine' },
    { frequency: 1319, offset: 0.23, duration: 0.28, volume: 0.07, type: 'sine' },
  ],
  'slots-bonus-land': [
    { frequency: 523, duration: 0.1, volume: 0.052, type: 'triangle' },
    { frequency: 1047, duration: 0.08, volume: 0.022, type: 'sine' },
    { frequency: 659, offset: 0.07, duration: 0.1, volume: 0.055, type: 'triangle' },
    { frequency: 1319, offset: 0.07, duration: 0.08, volume: 0.022, type: 'sine' },
    { frequency: 784, offset: 0.14, duration: 0.12, volume: 0.06, type: 'triangle' },
    { frequency: 1568, offset: 0.14, duration: 0.09, volume: 0.024, type: 'sine' },
    { frequency: 1047, offset: 0.22, duration: 0.14, volume: 0.065, type: 'triangle' },
    { frequency: 1319, offset: 0.31, duration: 0.16, volume: 0.065, type: 'triangle' },
    { frequency: 1568, offset: 0.4, duration: 0.18, volume: 0.07, type: 'triangle' },
    { frequency: 2093, offset: 0.49, duration: 0.36, volume: 0.075, type: 'sine' },
  ],
  'fruit-center-spin': FRUIT_CENTER_FALLBACK,
  'fruit-outer-spin': FRUIT_OUTER_FALLBACK,
  'fruit-reel-stop': [
    { frequency: 988, endFrequency: 659, duration: 0.1, volume: 0.055, type: 'square' },
    { frequency: 494, offset: 0.055, duration: 0.16, volume: 0.05, type: 'triangle' },
  ],
  'fruit-center-stop': [
    { frequency: 659, duration: 0.08, volume: 0.05, type: 'triangle' },
    { frequency: 988, offset: 0.06, duration: 0.11, volume: 0.055, type: 'sine' },
    { frequency: 1319, offset: 0.13, duration: 0.2, volume: 0.06, type: 'sine' },
  ],
  'fruit-win': [
    { frequency: 523, duration: 0.09, volume: 0.055, type: 'triangle' },
    { frequency: 659, offset: 0.08, duration: 0.1, volume: 0.06, type: 'triangle' },
    { frequency: 784, offset: 0.16, duration: 0.11, volume: 0.065, type: 'triangle' },
    { frequency: 1047, offset: 0.25, duration: 0.28, volume: 0.07, type: 'sine' },
  ],
  'fruit-jackpot': [
    { frequency: 392, duration: 0.12, volume: 0.055, type: 'triangle' },
    { frequency: 523, offset: 0.08, duration: 0.12, volume: 0.06, type: 'triangle' },
    { frequency: 659, offset: 0.16, duration: 0.14, volume: 0.065, type: 'triangle' },
    { frequency: 784, offset: 0.25, duration: 0.16, volume: 0.07, type: 'triangle' },
    { frequency: 1047, offset: 0.36, duration: 0.42, volume: 0.075, type: 'sine' },
  ],
  'fruit-no-win': [
    { frequency: 494, endFrequency: 392, duration: 0.11, volume: 0.04, type: 'triangle' },
    { frequency: 330, offset: 0.09, duration: 0.16, volume: 0.038, type: 'sine' },
  ],
  'fruit-bonus-enter': [
    { frequency: 523, duration: 0.09, volume: 0.05, type: 'triangle' },
    { frequency: 784, offset: 0.07, duration: 0.11, volume: 0.055, type: 'triangle' },
    { frequency: 1047, offset: 0.15, duration: 0.2, volume: 0.065, type: 'sine' },
  ],
  'fruit-bonus-land': [
    { frequency: 659, duration: 0.08, volume: 0.052, type: 'square' },
    { frequency: 988, offset: 0.06, duration: 0.12, volume: 0.06, type: 'triangle' },
    { frequency: 1319, offset: 0.14, duration: 0.2, volume: 0.065, type: 'sine' },
  ],
  'checkin-success': [
    { frequency: 523, duration: 0.08, volume: 0.07, type: 'sine' },
    { frequency: 784, offset: 0.07, duration: 0.12, volume: 0.08, type: 'sine' },
  ],
  'reward-success': [
    { frequency: 392, duration: 0.08, volume: 0.07, type: 'triangle' },
    { frequency: 523, offset: 0.07, duration: 0.09, volume: 0.075, type: 'triangle' },
    { frequency: 659, offset: 0.14, duration: 0.11, volume: 0.08, type: 'triangle' },
    { frequency: 784, offset: 0.22, duration: 0.18, volume: 0.09, type: 'sine' },
  ],
}

const THROTTLE_MS: Partial<Record<ArcadeSound, number>> = {
  move: 45,
  collect: 40,
  'tetris-rotate': 45,
  'tetris-drop': 80,
  'tetris-line-clear': 120,
  'breakout-brick': 32,
  'orbital-shoot': 70,
  'orbital-hit': 40,
  'orbital-life': 200,
  'hoarder-hit': 55,
  'slots-reel-tick': 55,
  'slots-spin': 120,
  'slots-reel-stop': 40,
  'slots-free-spins': 500,
  'slots-bonus-pick': 120,
  'slots-bonus-wheel': 200,
  'fruit-center-spin': 500,
  'fruit-outer-spin': 500,
  'fruit-reel-stop': 80,
  'fruit-center-stop': 120,
  'fruit-win': 500,
  'fruit-jackpot': 800,
  'fruit-no-win': 200,
  'fruit-bonus-enter': 300,
  'fruit-bonus-land': 150,
  'checkin-success': 200,
  'reward-success': 200,
}

function browserStorage(): StorageLike | null {
  try {
    return typeof window === 'undefined' ? null : window.localStorage
  } catch {
    return null
  }
}

export function readArcadeMuted(storage: StorageLike | null = browserStorage()): boolean {
  try {
    return storage?.getItem(AUDIO_MUTED_STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

function createBrowserAudioContext(): AudioContext {
  const BrowserAudioContext = window.AudioContext
    ?? (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
  if (!BrowserAudioContext) throw new Error('Web Audio is not supported')
  return new BrowserAudioContext({ latencyHint: 'interactive' })
}

function createBrowserMediaFactory(): ((source: string) => HTMLAudioElement) | null {
  if (import.meta.env.MODE === 'test' || typeof Audio === 'undefined') return null
  return (source: string) => new Audio(source)
}

export class ArcadeAudio {
  private readonly storage: StorageLike | null
  private readonly createContext: () => AudioContext
  private readonly now: () => number
  private readonly createMediaElement: ((source: string) => HTMLAudioElement) | null
  private readonly lastPlayedAt = new Map<ArcadeSound, number>()
  private readonly oscillators = new Set<OscillatorNode>()
  private readonly mediaClips = new Set<HTMLAudioElement>()
  private readonly mediaPreloads = new Map<string, HTMLAudioElement>()
  private context: AudioContext | null = null
  private masterGain: GainNode | null = null
  private unlockTarget: EventTarget | null = null
  private unlockPromise: Promise<void> | null = null
  private pendingSound: ArcadeSound | null = null
  private destroyed = false
  private muted: boolean

  constructor(options: ArcadeAudioOptions = {}) {
    this.storage = options.storage === undefined ? browserStorage() : options.storage
    this.createContext = options.createContext ?? createBrowserAudioContext
    this.now = options.now ?? (() => (typeof performance === 'undefined' ? Date.now() : performance.now()))
    this.createMediaElement = options.createMediaElement === undefined
      ? createBrowserMediaFactory()
      : options.createMediaElement
    this.muted = readArcadeMuted(this.storage)
  }

  get isMuted(): boolean {
    return this.muted
  }

  attachUnlockListeners(target: EventTarget): void {
    this.detachUnlockListeners()
    if (this.destroyed) return
    this.unlockTarget = target
    target.addEventListener('pointerdown', this.handleFirstInteraction, { capture: true, passive: true })
    target.addEventListener('keydown', this.handleFirstInteraction, true)
  }

  setMuted(muted: boolean): void {
    this.muted = muted
    if (muted) this.pendingSound = null
    if (muted) this.stopMediaClips()
    try {
      this.storage?.setItem(AUDIO_MUTED_STORAGE_KEY, muted ? '1' : '0')
    } catch {
      // Storage can be unavailable in privacy modes; the session setting still applies.
    }
    if (this.masterGain && this.context) {
      this.masterGain.gain.setValueAtTime(muted ? 0 : 0.72, this.context.currentTime)
    }
  }

  async unlock(): Promise<void> {
    if (this.destroyed || this.muted) return
    if (this.context?.state === 'running') {
      this.detachUnlockListeners()
      return
    }
    if (this.unlockPromise) return this.unlockPromise

    try {
      const context = this.ensureContext()
      this.primeContext(context)
      this.unlockPromise = (context.state === 'suspended' ? context.resume() : Promise.resolve())
        .then(() => {
          if (this.destroyed || context.state !== 'running') return
          this.detachUnlockListeners()
          const pendingSound = this.pendingSound
          this.pendingSound = null
          if (pendingSound) this.play(pendingSound)
        })
        .catch(() => undefined)
        .finally(() => {
          this.unlockPromise = null
        })
      return this.unlockPromise
    } catch {
      return
    }
  }

  play(sound: ArcadeSound): void {
    if (this.destroyed || this.muted || !this.context) return
    if (this.context.state !== 'running') {
      if (this.unlockPromise) this.pendingSound = sound
      return
    }

    if (!this.claimPlaybackSlot(sound)) return

    if (this.playMediaClip(sound)) return
    this.playTonePattern(sound)
  }

  preload(sounds: readonly ArcadeSound[]): void {
    if (this.destroyed || !this.createMediaElement) return
    sounds.forEach((sound) => {
      const clip = MEDIA_CLIPS[sound]
      if (!clip || this.mediaPreloads.has(clip.source)) return
      const media = this.createMediaElement?.(clip.source)
      if (!media) return
      media.preload = 'auto'
      media.load()
      this.mediaPreloads.set(clip.source, media)
    })
  }

  async playSynchronized(sound: ArcadeSound): Promise<GameAudioSyncAnchor> {
    const clip = MEDIA_CLIPS[sound]
    const fallback = (): GameAudioSyncAnchor => ({
      startedAtMs: this.now(),
      durationMs: clip?.durationMs ?? null,
    })
    if (this.destroyed || this.muted) return fallback()

    await this.unlock()
    if (!this.context || this.context.state !== 'running') return fallback()
    if (!this.claimPlaybackSlot(sound)) return fallback()

    if (clip && this.createMediaElement) {
      const anchor = await this.playSynchronizedMedia(clip)
      if (anchor) return anchor
    }
    this.playTonePattern(sound)
    return fallback()
  }

  destroy(): void {
    if (this.destroyed) return
    this.destroyed = true
    this.pendingSound = null
    this.detachUnlockListeners()
    this.oscillators.forEach((oscillator) => {
      try {
        oscillator.stop()
      } catch {
        // It may already have ended.
      }
      oscillator.disconnect()
    })
    this.oscillators.clear()
    this.stopMediaClips()
    this.mediaPreloads.forEach((media) => {
      media.pause()
      media.removeAttribute('src')
      media.load()
    })
    this.mediaPreloads.clear()
    this.masterGain?.disconnect()
    this.masterGain = null
    const context = this.context
    this.context = null
    if (context && context.state !== 'closed') void context.close().catch(() => undefined)
  }

  private readonly handleFirstInteraction = (): void => {
    void this.unlock()
  }

  private ensureContext(): AudioContext {
    if (this.context) return this.context
    const context = this.createContext()
    const masterGain = context.createGain()
    masterGain.gain.setValueAtTime(this.muted ? 0 : 0.72, context.currentTime)
    masterGain.connect(context.destination)
    this.context = context
    this.masterGain = masterGain
    return context
  }

  private primeContext(context: AudioContext): void {
    const oscillator = context.createOscillator()
    const gain = context.createGain()
    gain.gain.setValueAtTime(0, context.currentTime)
    oscillator.connect(gain)
    gain.connect(this.masterGain ?? context.destination)
    oscillator.start(context.currentTime)
    oscillator.stop(context.currentTime + 0.001)
    oscillator.onended = () => {
      oscillator.disconnect()
      gain.disconnect()
    }
  }

  private playTone(tone: Tone): void {
    const context = this.context
    const masterGain = this.masterGain
    if (!context || !masterGain) return

    const start = context.currentTime + (tone.offset ?? 0)
    const end = start + tone.duration
    const oscillator = context.createOscillator()
    const gain = context.createGain()
    oscillator.type = tone.type ?? 'sine'
    oscillator.frequency.setValueAtTime(tone.frequency, start)
    if (tone.endFrequency) oscillator.frequency.exponentialRampToValueAtTime(tone.endFrequency, end)
    gain.gain.setValueAtTime(0.0001, start)
    gain.gain.exponentialRampToValueAtTime(tone.volume, start + Math.min(0.008, tone.duration / 3))
    gain.gain.exponentialRampToValueAtTime(0.0001, end)
    oscillator.connect(gain)
    gain.connect(masterGain)
    this.oscillators.add(oscillator)
    oscillator.onended = () => {
      this.oscillators.delete(oscillator)
      oscillator.disconnect()
      gain.disconnect()
    }
    oscillator.start(start)
    oscillator.stop(end + 0.01)
  }

  private claimPlaybackSlot(sound: ArcadeSound): boolean {
    const now = this.now()
    const throttle = THROTTLE_MS[sound] ?? 0
    const lastPlayedAt = this.lastPlayedAt.get(sound) ?? Number.NEGATIVE_INFINITY
    if (now - lastPlayedAt < throttle) return false
    this.lastPlayedAt.set(sound, now)
    return true
  }

  private playTonePattern(sound: ArcadeSound): void {
    SOUND_PATTERNS[sound].forEach((tone) => this.playTone(tone))
  }

  private newMediaElement(clip: MediaClip): HTMLAudioElement | null {
    if (!this.createMediaElement) return null
    const template = this.mediaPreloads.get(clip.source)
    const media = template
      ? template.cloneNode(true) as HTMLAudioElement
      : this.createMediaElement(clip.source)
    if (!media.getAttribute('src')) media.src = clip.source
    media.preload = 'auto'
    media.volume = clip.volume
    return media
  }

  private playMediaClip(sound: ArcadeSound): boolean {
    const clip = MEDIA_CLIPS[sound]
    if (!clip) return false

    const media = this.newMediaElement(clip)
    if (!media) return false
    const cleanup = (): void => {
      this.mediaClips.delete(media)
      media.removeEventListener('ended', cleanup)
      media.removeEventListener('error', cleanup)
    }
    media.addEventListener('ended', cleanup, { once: true })
    media.addEventListener('error', cleanup, { once: true })
    this.mediaClips.add(media)
    void media.play().catch(cleanup)
    return true
  }

  private playSynchronizedMedia(clip: MediaClip): Promise<GameAudioSyncAnchor | null> {
    const media = this.newMediaElement(clip)
    if (!media) return Promise.resolve(null)

    this.mediaClips.add(media)
    return new Promise((resolve) => {
      let settled = false
      const timeout = setTimeout(() => finish(null, true), 1200)
      const cleanup = (): void => {
        clearTimeout(timeout)
        this.mediaClips.delete(media)
        media.removeEventListener('playing', onPlaying)
        media.removeEventListener('ended', onEnded)
        media.removeEventListener('error', onError)
      }
      const finish = (anchor: GameAudioSyncAnchor | null, stopMedia: boolean): void => {
        if (settled) return
        settled = true
        clearTimeout(timeout)
        if (stopMedia) {
          media.pause()
          media.currentTime = 0
        }
        if (!anchor) cleanup()
        resolve(anchor)
      }
      const onPlaying = (): void => {
        const durationMs = Number.isFinite(media.duration) && media.duration > 0
          ? Math.round(media.duration * 1000)
          : clip.durationMs ?? null
        finish({
          startedAtMs: this.now() - Math.max(0, media.currentTime * 1000),
          durationMs,
        }, false)
      }
      const onEnded = (): void => cleanup()
      const onError = (): void => {
        if (settled) {
          cleanup()
          return
        }
        finish(null, true)
      }

      media.addEventListener('playing', onPlaying, { once: true })
      media.addEventListener('ended', onEnded, { once: true })
      media.addEventListener('error', onError, { once: true })
      void media.play().catch(onError)
    })
  }

  private stopMediaClips(): void {
    this.mediaClips.forEach((media) => {
      try {
        media.pause()
        media.currentTime = 0
      } catch {
        // A browser may release an already-ended media element asynchronously.
      }
    })
    this.mediaClips.clear()
  }

  private detachUnlockListeners(): void {
    if (!this.unlockTarget) return
    this.unlockTarget.removeEventListener('pointerdown', this.handleFirstInteraction, true)
    this.unlockTarget.removeEventListener('keydown', this.handleFirstInteraction, true)
    this.unlockTarget = null
  }
}
