# R41 Reference Fruit Machine

Release: `0.1.165-r41`

## Goal

Rebuild the server-authoritative fruit machine from the supplied physical-machine
video. The result must keep the existing Sub2API game wallet, idempotency, ledger,
leaderboard, mute setting, and responsive game shell.

The visual direction is a coordinated modern restoration, not a photo pasted into
the canvas. Use the generated production symbols in:

`frontend/public/game-assets/fruit-machine/`

## Eight bet doors

Keep the current API keys for compatibility. Only their displayed names change.

| API key | Display | Asset |
|---|---|---|
| `bar` | BAR | `bar.webp` |
| `77` | 9 | `nine.webp` |
| `star` | 星星 | `star.webp` |
| `watermelon` | 西瓜 | `watermelon.webp` |
| `bell` | 铃铛 | `bell.webp` |
| `mango` | 李子 | `plum.webp` |
| `orange` | 橙子 | `orange.webp` |
| `apple` | 苹果 | `apple.webp` |

## Outer 24-cell board

Indices run clockwise. The geometry is 7 cells across the top, 5 additional cells
down the right, 7 cells right-to-left across the bottom, and 5 additional cells up
the left. This is deliberately `7 + 5 + 7 + 5`, not the old `8 + 4 + 8 + 4`.

| Index | Cell | Multiplier |
|---:|---|---:|
| 0 | 橙子大图 | 20 |
| 1 | 铃铛大图 | 20 |
| 2 | BAR | 60 |
| 3 | BAR | 120 |
| 4 | BAR | 80 |
| 5 | 苹果大图 | 6 |
| 6 | 李子大图 | 20 |
| 7 | 西瓜大图 | 40 |
| 8 | 西瓜小图 | 3 |
| 9 | 金猪特殊格 | special |
| 10 | 苹果大图 | 6 |
| 11 | 橙子小图 | 3 |
| 12 | 橙子大图 | 20 |
| 13 | 铃铛大图 | 20 |
| 14 | 9 小图 | 3 |
| 15 | 9 大图 | 40 |
| 16 | 苹果大图 | 6 |
| 17 | 李子小图 | 3 |
| 18 | 李子大图 | 20 |
| 19 | 星星大图 | 40 |
| 20 | 星星小图 | 3 |
| 21 | 金猪特殊格 | special |
| 22 | 苹果大图 | 6 |
| 23 | 铃铛小图 | 3 |

The physical artwork prints ranges (`5-6`, `10-20`, `20-40`), while the supplied
video settles at the upper configured values: `6`, `20`, and `40`. The web UI may
show the printed range as secondary copy, but the highlighted current payout must
show the real configured multiplier above.

## Center 16-cell wheel

Clockwise from the top:

1. 金猪再转
2. 9
3. 双响炮
4. 星星
5. 彩金
6. 西瓜
7. 大四喜
8. 铃铛
9. 谢谢有奖
10. 李子
11. 小三元
12. 橙子
13. 大三元
14. 苹果
15. 开火车
16. BAR

R41 keeps the existing server-owned special outcomes and maps them visibly:

- `send_light` -> center lands on `开火车`, then displays its reward stops.
- `little_mary` -> center lands on `小三元`, then displays its reward stops.
- `jackpot` -> center lands on `彩金`.

Do not invent a client-side payout for dormant center labels. They are visual board
positions until a later server rules revision activates them.

## Frontend requirements

- Rewrite only the fruit scene and its focused tests unless a small shared type or
  audio addition is strictly needed.
- Preload and use all eight WebP symbols. Do not replace them with emoji or text.
- Keep the internal Phaser canvas at 900 x 620 and let the existing FIT scale handle
  desktop/mobile. All labels must remain legible at mobile width.
- Build a red/gold/ivory/dark-green cabinet consistent with Sub2API. No nested cards,
  purple gradient theme, decorative blobs, or text overlap.
- Top HUD: `本局赢分`, `积分`, and a central JP medallion.
- Bottom: eight stable bet doors in the reference order, then compact `清注`, `续押`,
  and `启动` controls.
- Keep server authority. The animation always lands on `stop_index` from the API.
- Use two animation phases for special outcomes: outer ring stops on 9 or 21, then
  the center wheel runs and lands on the mapped result.
- Reduced-motion mode must still show both authoritative final stops.
- Preserve pause, retry bet restoration, insufficient-credit handling, and wallet
  refresh behavior.

## Audio requirements

Fruit-specific media clips live under `frontend/public/game-audio/`. They must not
replace sounds used by the five-reel slot game. Add fruit-specific sound event names
where needed and retain synthesized tones as fallback.

The completed R41 timing follows the supplied machine recording:

- `fruit-center.mp3`: 7.30 seconds for the 65-step center prelude.
- `fruit-outer.mp3`: 5.90 seconds for at least three complete outer laps, deceleration,
  and the final landing flash.
- `fruit-win.mp3`: 2.25 seconds for fruit-machine settlement.
- Fruit audio events are independent from all existing `slots-*` events and retain
  Web Audio synthesis as a load/playback fallback.

## Backend requirements

- Use `fruit-v2` for new spins; never mutate historical `fruit-v1` rows.
- Keep request keys `bar`, `77`, `star`, `watermelon`, `bell`, `mango`, `orange`,
  `apple` so existing clients and idempotent replays remain valid.
- Return a real outer stop index matching the table above.
- Preserve server-side payout calculation, JSON outcome audit data, wallet locking,
  immutable spin rows, and replay-before-RNG behavior.
- Add tests for all 24 cells, visible multipliers, total virtual weight, payout math,
  special outer stops, RTP bounds, and historical replay.

## Verification

- Focused Go service, repository, and migration tests: 9 passed.
- Focused frontend scene/audio tests: 8 passed.
- Frontend `vue-tsc --noEmit` and production Vite build passed.
- The production build contains all three clips under
  `backend/internal/web/dist/game-audio/`.
- Interactive QA passed at 1280 x 900 and 390 x 844 with no horizontal overflow.
  BAR betting, start, center prelude, moving outer lamp, authoritative index 3 stop,
  and BAR x120 settlement were all observed.
- Production audio range requests return HTTP 206 with `audio/mpeg` for all three
  clips. Existing five-reel slot mappings remain unchanged.

## Deployment

- Deployed binary SHA256:
  `6ea4ac23fab0081dc66e4a2c5adb02f965b38fc7ec272e932b4af324cbb06fc5`.
- R40 rollback backup:
  `/opt/sub2api/backups/sub2api-r40-before-r41-20260731T011733Z`.
- Service state after deployment: `active/running`, `NRestarts=0`.
- Private and public `/health` checks return HTTP 200.
