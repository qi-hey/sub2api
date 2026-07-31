# Game Center R20

## Scope

R20 adds an authenticated game center, seven Phaser games, synthesized game
audio, and a server-owned game wallet to Sub2API.

Routes:

- `/games`: game catalog and wallet exchange UI
- `/games/:gameId`: playable game canvas

Games:

- Snake
- Tetris
- Breakout
- 2048
- Orbital Defense
- Hoarder
- Lucky Slots

All games use a fixed `900x620` logical canvas with responsive FIT scaling,
desktop keyboard controls, mobile touch controls, and a 60 FPS target. Phaser is
route-lazy-loaded in its own `vendor-phaser` chunk, so normal Sub2API pages do
not download the game engine.

Game sounds are synthesized with Web Audio. The implementation unlocks audio
on the first user interaction, rate-limits frequent effects, persists mute
state, and releases the audio context and listeners when leaving a game.

## Scores And Credits

Best scores and the original Lucky Slots demo credits remain browser-local and
have no monetary value. They must never be accepted by the backend as proof of
redeemable credits.

Redeemable game credits are stored only in `game_wallets`. Balance exchanges
are disabled by default and require an administrator to configure a positive
integer exchange rate before enabling the feature. R20 does not let client-side
game outcomes mint server credits.

Settings:

- `game_wallet_enabled` (default `false`)
- `game_wallet_exchange_rate` (integer credits per one balance unit, default `0`)
- `game_wallet_daily_limit` (balance units across both directions, `0` is unlimited)

User API:

- `GET /api/v1/user/game-wallet`
- `POST /api/v1/user/game-wallet/exchange`
- `GET /api/v1/user/game-wallet/transactions`

Every exchange requires an `Idempotency-Key`. The repository locks the user
before the wallet, checks both source balances, performs decimal conversion in
PostgreSQL, updates both assets, and records before/after snapshots in one
transaction. Returning credits to balance does not update `total_recharged` or
run affiliate/recharge logic.

The write endpoint is limited to 30 requests per authenticated user per minute
and fails closed if Redis is unavailable. A second database guard permits at
most 1,000 exchanges per user per day. Successful exchanges invalidate API-key
authentication and balance caches. Request amounts accept exact JSON strings or
legacy JSON numbers, but reject exponents, signs, overlong values, fractional
credits, and unsupported precision. IDs and credit amounts are serialized as
strings so browser clients do not lose `BIGINT` precision.

The exchange feature must remain disabled in production until the operator has
explicitly selected and approved an exchange rate and daily limit:

- `game_wallet_enabled=false`
- `game_wallet_exchange_rate=0`
- `game_wallet_daily_limit=0`

Orbital Defense uses a bounded 32-object enemy pool. The shared canvas captures
arrow-key scrolling only while focused and owns the `P` pause/resume shortcut,
which remains available while a Phaser scene is paused. Every canvas exposes
the localized control description through `aria-describedby`.

## Upgrade Surface

Keep these paths and integrations when rebasing onto a newer upstream release:

- `frontend/src/features/game-center/**`
- `frontend/src/views/user/GameCenterView.vue`
- `frontend/src/views/user/GamePlayView.vue`
- `frontend/src/features/game-center/components/GameCanvas.vue`
- `frontend/src/features/game-center/__tests__/{GameCanvas,orbital}.spec.ts`
- `frontend/src/views/user/__tests__/GamePlayView.spec.ts`
- `frontend/src/views/user/__tests__/GameCenterView.spec.ts`
- `frontend/src/assets/game-center/arcade-hall.webp`
- `frontend/src/api/gameWallet.ts` and its exports/tests
- `frontend/src/types/gameWallet.ts` and its type exports
- `frontend/src/i18n/locales/{zh,en}/games.ts`
- the game wallet fields in admin settings API, view, locales, and tests
- the `/games` routes in `frontend/src/router/index.ts`
- the Game Center entries in `frontend/src/components/layout/AppSidebar.vue`
- the `nav.gameCenter` locale keys
- the `phaser` dependency and lockfile entry
- the `vendor-phaser` rule in `frontend/vite.config.ts`
- `backend/migrations/191_game_wallets.sql` and its migration test
- `backend/internal/service/game_wallet.go` and tests
- `backend/internal/repository/game_wallet_repo.go` and tests
- `backend/internal/handler/game_wallet_handler.go`, DTO, routes, and tests
- game wallet settings across service, DTO, admin handler, and audit mapping
- repository/service/handler Wire providers and generated `wire_gen.go`

## Verification

Before deployment:

1. Run the full frontend Vitest suite and `pnpm --dir frontend run typecheck`.
2. Run `pnpm --dir frontend run build` and verify Phaser is not referenced by
   `dist/index.html` and exists as a separate `vendor-phaser` chunk.
3. Run wallet service, repository, handler, settings, and migration tests.
4. Run `go test ./... -run '^$'` and relevant `go vet` packages.
5. Verify desktop and mobile layouts, canvas pixels, controls, mute persistence,
   and audio cleanup in a real browser.
6. Build the Linux binary with `scripts/build-linux.sh <version>`.
7. Back up the binary and database, then verify `/health`, login, API keys,
   existing API traffic, `/games`, and every game route after deployment.

## Rollback

Restore the previous binary and restart `sub2api.service`. Migration 191 is
additive and its settings default to disabled, so the old binary ignores the
new tables. Keep the wallet tables and transaction ledger for audit purposes;
do not drop them during a routine binary rollback.
