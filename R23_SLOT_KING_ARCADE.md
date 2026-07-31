# Sub2API 0.1.165-r23 Slot King arcade

## Scope

R23 keeps all R21 and R22 customizations and upgrades Lucky Slots into a
Chinese arcade-cabinet experience inspired by classic five-reel machines:

- Bright purple, red, and gold 5x3 cabinet with readable symbols, reel stops,
  win-line highlights, responsive controls, synthesized sound, and reduced-motion support.
- A separate paytable beside the cabinet on desktop and below it on mobile.
- Independent WILD, SCATTER, and BONUS behavior.
- SCATTER awards scatter payout plus 5, 8, or 12 free spins for 3, 4, or 5 symbols.
- Three or more BONUS symbols may open a server-issued second-screen game.
- Bonus rounds support a three-choice treasure game and a server-settled lucky wheel.

## Trust boundaries

- Reel stops, the visible grid, line wins, scatter wins, free spins, bonus eligibility,
  bonus type, bonus rewards, wallet credits, ledger entries, and leaderboard scores are
  authoritative backend results.
- The frontend only animates the returned result. It never chooses a lucky-wheel reward.
- Bonus tokens remain user-bound, short-lived, single-use, and stored only as SHA-256 hashes.
- Bonus wallet updates, immutable ledger writes, round state, and Lucky leaderboard updates
  remain in one database transaction.
- The existing one-bonus-round-per-user-per-local-day constraint remains the hard reward cap.

## Compatibility

- R22 bonus rows stored `rewards` as a JSON array. R23 accepts both the legacy array and the
  R23 object form containing the bonus kind and values, so pending R22 rounds remain claimable.
- Existing game wallet, check-in, spin, reward, redeem-code, group, API-key, account, routing,
  privacyfilter, proxy, and session data are not removed or rewritten.
- R23 does not require a destructive migration; the existing JSONB rewards column carries the
  versioned bonus payload.

## API additions

The existing slot spin response adds:

- `bonus_count`: visible BONUS symbol count.
- `bonus_round.kind`: `pick_chest` or `lucky_wheel` when a round is issued.

The existing bonus claim endpoint and idempotency contract remain unchanged.

## Upgrade preservation checklist

When rebasing onto a later Sub2API release, preserve:

1. The R21 and R22 game wallet, reward, leaderboard, bonus, and security boundaries.
2. The independent BONUS symbol and SCATTER-only free-spin behavior.
3. Legacy R22 rewards-array decoding and deterministic bonus-token replay.
4. The `bonus_count` and `bonus_round.kind` API fields across service, repository, DTO, and frontend types.
5. The server-settled treasure and lucky-wheel flows; frontend animation must never mint credits.
6. The desktop side paytable and mobile below-cabinet layout.
7. Chinese copy, sound, reduced-motion support, restart guards, and transport retry idempotency.
8. All unrelated R11-R22 custom routing, Grok, proxy, privacyfilter, backup, and account defaults.

## Rollback

The R22 binary can run against an R23 database because R23 uses the existing JSONB rewards column
and does not drop or alter R22 tables. Keep the pre-deploy binary, source archive, and database dump
until browser, wallet, and API acceptance checks pass.

## Production deployment

- Deployed version: `0.1.165-r23` (`custom-r23`) on 2026-07-29 China Standard Time.
- Production binary SHA-256: `96ad66da62977bfb795e35d9850ac9cdc05f10e1023d35797631b6543c101c0f`.
- Pre-deploy backup: `/opt/sub2api/backups/r23-20260728T170858Z`.
- Migration `194_game_slot_bonus_v2.sql` was applied transactionally.
- Full frontend and backend test suites passed before deployment.
- Production verification: service active, `NRestarts=0`, root and game routes return HTTP 200,
  no panic or fatal log entries after deployment.
- Browser verification: 1440x1000 keeps the paytable beside the cabinet; 390x844 moves it below,
  with no overlap or horizontal overflow. The mobile cabinet canvas is 343x231 CSS pixels and its
  captured image entropy is 7.363, confirming a nonblank rendered scene.
