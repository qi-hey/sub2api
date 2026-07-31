# R45 Fruit HD Rendering and Audio Sync

Release: `0.1.165-r45`

R45 sharpens the fruit machine on large and high-DPI screens and keeps both
lamp wheels tied to the actual soundtrack clock.

## Rendering

- The fruit machine now creates a high-DPI Phaser backing canvas from its CSS
  size and the device pixel ratio, capped at 2x for stable performance.
- World coordinates remain `900x620`, so cabinet layout, hit targets, and game
  settlement behavior are unchanged.
- Text textures use the same render scale as the canvas. Other games keep their
  existing rendering path.

## Audio timing

- Fruit-machine soundtrack files are preloaded when the game opens.
- Lamp animation starts from the media element's real `playing` timestamp, not
  from the earlier asynchronous `play()` request.
- Each lamp step uses an absolute soundtrack deadline, preventing delayed
  browser timers from accumulating drift.
- Muted, blocked, or unavailable media keeps the existing synthesized-audio and
  animation fallback, so a sound failure cannot block a spin.

Wallet settlement, odds pools, paytables, game rules, histories, and all R44
behavior are unchanged.
