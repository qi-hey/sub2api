# Sub2API 0.1.165-r24 joyful slot audio

## Scope

R24 preserves R23 and replaces only the Lucky Slots sound palette with an original,
cheerful Chinese arcade-style arrangement:

- Bright rolling ticks and a rising spin-start phrase.
- Bell-like reel stops with less mechanical bass.
- Major-key win and jackpot fanfares with octave shimmer.
- A dedicated free-spin celebration cue.
- Richer treasure-pick, wheel, reveal, and bonus-land cues.
- A short neutral no-win cue that does not sound punitive.

## Boundaries

- No copied or extracted commercial game audio is bundled.
- Payouts, probabilities, reels, wallets, bonus settlement, and API behavior are unchanged.
- All sounds continue to use the existing Web Audio runtime, mute persistence, mobile
  interaction unlock, throttling, and cleanup behavior.
- Other games keep their existing sound patterns.

## Upgrade preservation checklist

Preserve the `slots-free-spins` event, the R24 slot patterns in `audio.ts`, the R23
BONUS and SCATTER separation, and every earlier R11-R23 customization.

## Deployment record

- Deployed version: `0.1.165-r24` (`custom-r24`).
- Deployed at: `2026-07-29T00:49:20Z`.
- Previous binary backup: `/opt/sub2api/backups/r24-20260729T004840Z/sub2api-r23`.
- VPS build directory: `/opt/sub2api/builds/0.1.165-r24`.
- Verification: backend `go test ./...`, embedded production build, `/games` HTTP 200,
  service `active/running`, `NRestarts=0`, and no error-level journal entries after restart.
