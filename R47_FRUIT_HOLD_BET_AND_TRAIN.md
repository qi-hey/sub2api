# R47 Fruit Hold-to-bet and Train Animation

Release: `0.1.165-r47`

R47 makes each fruit-machine betting panel support press-and-hold input while
keeping ordinary single-click betting unchanged.

- Pointer down adds one credit immediately.
- Holding for 300 ms starts adding one credit every 60 ms.
- Releasing, leaving the panel or game, pausing, clearing, repeating, starting a
  spin, restarting, or closing the scene stops the repeat timer immediately.
- Each door keeps the existing 100-credit maximum.
- Touch callouts and page scrolling are disabled only over the fruit-machine
  canvas so long presses work consistently on phones.
- Send-light wins now render a five-car light train moving clockwise cell by
  cell around the 24-cell board. The first reward runs at least two laps and
  each following reward continues for at least one lap before landing.
- Every authoritative reward stop is shown in order with progress text and its
  server-provided multiplier. Reduced-motion mode keeps the direct landing.

Settlement, wallet data, paytables, odds pools, histories, R44 controls, R45
HD/audio synchronization behavior, and the deployed R46 baseline are unchanged.

## Verification and deployment

- Focused fruit-machine suite: 7 tests passed.
- Full frontend Vitest suite, TypeScript check, ESLint, and Vite production
  build passed.
- Full Go repository compile check passed with `-tags embed`.
- Embedded root and SPA route tests passed. Two pre-existing static-file cases
  still expect the removed `/logo.png` asset and are unrelated to R47.
- Linux amd64 binary:
  `artifacts/sub2api-0.1.165-r47-linux-amd64`
- SHA256:
  `15196429a75e8af45029b3e215848db482d3e97902147acbc8fc3c95717a1518`
- VPS rollback binary:
  `/opt/sub2api/backups/sub2api-0.1.165-r46-before-r47-20260731T134242Z`
- Post-deploy checks: `/health`, `/`, `/games`, and `/admin/usage` returned
  HTTP 200; systemd reported `active/running` with `NRestarts=0`.
- Chrome desktop and 390 x 844 mobile checks confirmed a nonblank, fitted
  canvas. A 720 ms hold added eight credits on both viewports and stopped
  immediately after release.
