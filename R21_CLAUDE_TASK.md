# R21 Loyalty Casino Task

## Objective

Replace the R20 balance exchange concept with a server-authoritative loyalty
system:

1. One daily check-in grants entertainment credits.
2. Credits fund a responsive 5x3, 20-payline casino-style slot.
3. Slot wins return credits according to a fixed, versioned paytable.
4. Credits buy `game_token` vouchers that add API usage balance only.

The production feature must fail closed and remain disabled until the operator
sets the economics explicitly.

## Invariants

- R20 tables and data remain intact for rollback.
- The R20 `/game-wallet/exchange` route and UI are removed in R21.
- Other games never grant redeemable credits.
- Check-in, spin, and reward claim mutations are atomic, idempotent, rate
  limited, concurrency safe, and recorded in immutable tables.
- Credits are integer `BIGINT` values and JSON responses serialize them as
  strings.
- Voucher values use exact decimal strings. No float arithmetic is allowed in
  the R21 transaction path.
- Slot outcomes and payouts are generated and evaluated only by the server.
- RNG uses `crypto/rand`, rejection sampling, and an injectable reader for
  tests. Seeds are never returned to clients or persisted in plaintext.
- `game_token` vouchers add API usage balance without changing
  `total_recharged` and without affiliate rebate.
- Existing users keep their current `game_wallets.credits`.

## Data

Migration `192_game_loyalty_rewards.sql` adds:

- `game_credit_ledger`: immutable signed checkin, slot_bet, slot_payout, and
  reward_claim entries with before/after values, reference, idempotency key,
  request hash, JSON metadata, and timestamp.
- `game_daily_checkins`: immutable unique `(user_id, local_date)` claims.
- `game_slot_spins`: immutable authoritative bet, result, winning lines,
  versions, credits before/after, request hash, and idempotency key.
- `game_reward_claims`: immutable catalog snapshot, exact voucher value,
  generated redeem code, claim date, and idempotency key.
- `game_wallets.free_spins_remaining` for server-owned free-spin state.
- Disabled `game_loyalty_*` settings.

Use table-specific immutable trigger functions. Do not introduce a generic
function name that can collide with upstream migrations. Do not use unsupported
`ALTER TABLE ... ADD CONSTRAINT IF NOT EXISTS` syntax.

## API

- `GET /api/v1/user/game-wallet`
- `POST /api/v1/user/game-wallet/check-in`
- `POST /api/v1/user/game-wallet/slot/spin`
- `GET /api/v1/user/game-wallet/rewards`
- `POST /api/v1/user/game-wallet/rewards/claim`
- `GET /api/v1/user/game-wallet/transactions`

Mutating requests require an `Idempotency-Key` header. A replay returns the
original response; reuse with another payload returns a conflict.

## Slot v1

- Five reels, three visible rows, twenty fixed paylines.
- Versioned reel strips and paytable.
- Symbols include jackpot 7, dragon, ingot, jade, bell, WILD, and SCATTER.
- WILD substitutes for normal line symbols. SCATTER pays anywhere and can award
  server-owned free spins. Free spins cannot be requested by the client.
- A spin response includes the result grid, stops, winning lines, bet, payout,
  remaining free spins, and authoritative credits. It excludes RNG seed.
- Paytable/reel-strip tests must estimate and document theoretical RTP. Target
  a conservative entertainment range and prevent runtime RTP mutation.

## Rewards

The catalog is validated JSON in settings. Each enabled item has a stable ID,
localized title, positive credit cost, exact positive API balance value, daily
stock, and optional expiry days.

Claiming a reward locks the wallet, enforces global and per-item daily caps,
deducts credits, and inserts both `redeem_codes(type='game_token')` and the
claim record in one PostgreSQL transaction. Generated codes must fit the actual
database column length and have enough entropy.

Teach the existing redeem path to process `game_token` through an exact balance
adjustment that does not update `total_recharged` and does not invoke affiliate
rebate. Invalidate auth and billing caches after successful redemption.

## Frontend

- All visible interface and game text must be Simplified Chinese, including
  controls, states, errors, accessibility labels, and slot symbol labels.
- Replace the R20 exchange panel with points, check-in, and reward store views.
- Replace the local three-reel Lucky scene with a black/red/gold 5x3 cabinet.
- The scene may animate anticipation locally but must stop on the exact server
  grid and display only the server payout.
- Support desktop keyboard and mouse plus mobile 52px touch controls.
- Preserve Web Audio unlock, mute persistence, reduced motion, and accessible
  labels.
- Other games stay unchanged and local-only.

## Verification

- Migration contract and PostgreSQL integration tests.
- Service/repository/handler tests covering idempotency, row locking, overflow,
  insufficient credits, duplicate check-in, timezone boundary, RNG failures,
  invalid settings, reward caps, voucher atomicity, and rollback.
- `game_token` redemption tests proving no recharge accumulation and no
  affiliate call.
- Frontend API, wallet UI, Lucky scene, keyboard, mobile, audio, and reduced
  motion tests.
- Full Go tests/compile/vet, Vitest, vue-tsc, production build, desktop/mobile
  browser screenshots, and live VPS health/API regression checks.
