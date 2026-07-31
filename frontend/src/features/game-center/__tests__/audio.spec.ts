import { describe, expect, it, vi } from 'vitest'
import { AUDIO_MUTED_STORAGE_KEY, ArcadeAudio, readArcadeMuted } from '../audio'

function createAudioContext() {
  const parameter = {
    setValueAtTime: vi.fn(),
    exponentialRampToValueAtTime: vi.fn(),
  }
  const oscillator = {
    type: 'sine',
    frequency: parameter,
    connect: vi.fn(),
    disconnect: vi.fn(),
    start: vi.fn(),
    stop: vi.fn(),
    onended: null,
  }
  const gain = {
    gain: parameter,
    connect: vi.fn(),
    disconnect: vi.fn(),
  }
  const context = {
    state: 'suspended',
    currentTime: 1,
    destination: {},
    createGain: vi.fn(() => ({ ...gain })),
    createOscillator: vi.fn(() => ({ ...oscillator, frequency: { ...parameter } })),
    resume: vi.fn(async () => { context.state = 'running' }),
    close: vi.fn(async () => { context.state = 'closed' }),
  }
  return context
}

describe('ArcadeAudio', () => {
  it('persists mute state', () => {
    const storage = { getItem: vi.fn(() => '1'), setItem: vi.fn() }
    expect(readArcadeMuted(storage)).toBe(true)
    const audio = new ArcadeAudio({ storage, createContext: vi.fn() })
    audio.setMuted(false)
    expect(storage.setItem).toHaveBeenCalledWith(AUDIO_MUTED_STORAGE_KEY, '0')
  })

  it('unlocks on first pointer interaction and throttles noisy effects', async () => {
    const context = createAudioContext()
    const factory = vi.fn(() => context as unknown as AudioContext)
    let now = 1000
    const audio = new ArcadeAudio({ storage: null, createContext: factory, now: () => now })
    const target = new EventTarget()
    audio.attachUnlockListeners(target)

    expect(factory).not.toHaveBeenCalled()
    target.dispatchEvent(new Event('pointerdown'))
    await Promise.resolve()
    const afterUnlock = context.createOscillator.mock.calls.length
    audio.play('move')
    audio.play('move')
    expect(context.createOscillator).toHaveBeenCalledTimes(afterUnlock + 1)
    now += 50
    audio.play('move')
    expect(context.createOscillator).toHaveBeenCalledTimes(afterUnlock + 2)
  })

  it('closes its AudioContext on destroy', async () => {
    const context = createAudioContext()
    const audio = new ArcadeAudio({ storage: null, createContext: () => context as unknown as AudioContext })
    await audio.unlock()
    audio.destroy()
    expect(context.close).toHaveBeenCalledOnce()
  })

  it('plays the joyful slot spin, tiered win, free-spin, and bonus-game cues after unlock', async () => {
    const context = createAudioContext()
    const audio = new ArcadeAudio({ storage: null, createContext: () => context as unknown as AudioContext })
    await audio.unlock()
    const before = context.createOscillator.mock.calls.length

    audio.play('slots-spin')
    audio.play('slots-reel-stop')
    audio.play('slots-win')
    audio.play('slots-big-win')
    audio.play('slots-jackpot')
    audio.play('slots-free-spins')
    audio.play('slots-bonus-enter')
    audio.play('slots-bonus-pick')
    audio.play('slots-bonus-wheel')
    audio.play('slots-bonus-reveal')
    audio.play('slots-bonus-land')

    expect(context.createOscillator.mock.calls.length).toBeGreaterThan(before + 50)
  })

  it('keeps dedicated fruit-machine center, outer, landing, and settlement fallbacks', async () => {
    const context = createAudioContext()
    const audio = new ArcadeAudio({ storage: null, createContext: () => context as unknown as AudioContext })
    await audio.unlock()
    const before = context.createOscillator.mock.calls.length

    audio.play('fruit-center-spin')
    audio.play('fruit-outer-spin')
    audio.play('fruit-reel-stop')
    audio.play('fruit-center-stop')
    audio.play('fruit-bonus-enter')
    audio.play('fruit-bonus-land')
    audio.play('fruit-win')
    audio.play('fruit-jackpot')
    audio.play('fruit-no-win')

    expect(context.createOscillator.mock.calls.length).toBeGreaterThan(before + 100)
  })

  it('anchors synchronized fruit animation to the real media playback clock', async () => {
    const context = createAudioContext()
    const media = new EventTarget() as HTMLAudioElement & {
      play: ReturnType<typeof vi.fn>
      pause: ReturnType<typeof vi.fn>
      load: ReturnType<typeof vi.fn>
    }
    Object.assign(media, {
      preload: '', volume: 1, currentTime: 0.125, duration: 5.9, src: '/fruit-outer.mp3',
      play: vi.fn(async () => { queueMicrotask(() => media.dispatchEvent(new Event('playing'))) }),
      pause: vi.fn(), load: vi.fn(),
      getAttribute: vi.fn(() => '/fruit-outer.mp3'),
      removeAttribute: vi.fn(),
    })
    const audio = new ArcadeAudio({
      storage: null,
      createContext: () => context as unknown as AudioContext,
      createMediaElement: () => media,
      now: () => 2000,
    })
    await audio.unlock()

    const anchor = await audio.playSynchronized('fruit-outer-spin')

    expect(anchor).toEqual({ startedAtMs: 1875, durationMs: 5900 })
    expect(media.play).toHaveBeenCalledOnce()
  })
})
