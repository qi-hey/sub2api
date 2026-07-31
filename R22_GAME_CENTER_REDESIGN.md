# Sub2API 0.1.165-r22 game center redesign

## Scope

R22 keeps every R21 customization and extends the game center with:

- A game-first layout with shortcuts to profile, check-in, credit ledger, and redeem-code usage.
- Daily leaderboards for all seven games, with Lucky Slots selected by default.
- Explicit Chinese insufficient-credit feedback before and after a spin request.
- A redesigned five-reel Lucky Slots cabinet with responsive sizing, win effects, and audio.
- A server-authoritative three-choice bonus round triggered by three or more scatter symbols.

## Trust boundaries

- Slot outcomes, bets, payouts, free spins, bonus choices, and redeemable credits are settled by the backend.
- Lucky Slots leaderboard scores are written only by backend settlement.
- The other six games run locally. Their leaderboard entries are entertainment scores and never change
  wallet credits, account balance, voucher stock, or redeem codes.
- Bonus claim tokens are user-bound, short-lived, single-use, and stored only as SHA-256 hashes.
- Bonus claims use the same transaction as the wallet update, immutable ledger entry, round state update,
  and Lucky Slots leaderboard update.

## Database migration

Migration `193_game_center_leaderboard_bonus.sql` adds:

- `game_daily_scores`
- `game_slot_bonus_rounds`
- `slot_bonus` to the game-credit ledger entry type constraint

The migration does not remove or rewrite R21 wallet, check-in, spin, reward, redeem-code, account, API-key,
group, or session data.

## API additions

- `GET /api/v1/user/game-wallet/leaderboard`
- `POST /api/v1/user/game-wallet/leaderboard/score`
- `POST /api/v1/user/game-wallet/slot/bonus/claim`

All endpoints require the existing authenticated user session. Mutation endpoints require an idempotency
key and use user-scoped fail-closed rate limiting.

## Upgrade preservation checklist

When rebasing R22 onto a later upstream release, preserve:

1. Migration 193 and its migration test.
2. The game-center repository and service contracts for leaderboard and bonus settlement.
3. User routes, DTOs, API client types, and Chinese error mappings.
4. The game-first center layout, quick links, leaderboard tabs, and Lucky Slots scene.
5. The rule that client-submitted Lucky Slots scores are rejected.
6. The rule that local-game scores never mint credits or rewards.
7. R21 reward catalog, check-in, wallet exchange, redeem-code creation, privacyfilter, routing, and proxy customizations.

## Rollback

The R21 binary can run while the two R22 tables remain present. A binary rollback therefore does not require
dropping R22 tables. Keep the pre-deploy binary and database dump until R22 has passed browser and API
acceptance checks.
